package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/AGlElectronics/suppervisor/internal/app"
	"github.com/AGlElectronics/suppervisor/internal/config"
	"github.com/AGlElectronics/suppervisor/internal/store"
)

type Bot struct {
	Config  config.Config
	Service app.Service
	Store   *store.Postgres
	Client  *http.Client
	Logger  *slog.Logger
}

type update struct {
	UpdateID int64 `json:"update_id"`
	Message  *struct {
		Text string `json:"text"`
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		From struct {
			ID int64 `json:"id"`
		} `json:"from"`
	} `json:"message"`
}

func (b Bot) WebhookHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if b.Config.TelegramWebhookSecret == "" || r.Header.Get("X-Telegram-Bot-Api-Secret-Token") != b.Config.TelegramWebhookSecret {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var item update
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&item); err != nil {
			http.Error(w, "invalid update", http.StatusBadRequest)
			return
		}
		go b.handle(context.Background(), item)
		w.WriteHeader(http.StatusNoContent)
	})
}

func (b Bot) ConfigureWebhook(ctx context.Context) error {
	if b.Config.TelegramWebhookURL == "" || b.Config.TelegramWebhookSecret == "" {
		return fmt.Errorf("webhook URL and secret are required")
	}
	return b.call(ctx, "setWebhook", map[string]any{"url": b.Config.TelegramWebhookURL, "secret_token": b.Config.TelegramWebhookSecret}, nil)
}

func (b Bot) RunPolling(ctx context.Context) error {
	var offset int64
	for ctx.Err() == nil {
		var response struct {
			OK     bool     `json:"ok"`
			Result []update `json:"result"`
		}
		err := b.call(ctx, "getUpdates", map[string]any{"offset": offset, "timeout": 30, "allowed_updates": []string{"message"}}, &response)
		if err != nil {
			b.logger().ErrorContext(ctx, "telegram poll failed", "error", err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2 * time.Second):
			}
			continue
		}
		for _, item := range response.Result {
			offset = item.UpdateID + 1
			b.handle(ctx, item)
		}
	}
	return ctx.Err()
}

func (b Bot) handle(ctx context.Context, item update) {
	if item.Message == nil {
		return
	}
	claimed, err := b.Store.ClaimTelegramUpdate(ctx, item.UpdateID)
	if err != nil {
		b.logger().ErrorContext(ctx, "telegram update claim failed", "update_id", item.UpdateID, "error", err)
		return
	}
	if !claimed {
		b.logger().InfoContext(ctx, "telegram duplicate ignored", "update_id", item.UpdateID)
		return
	}
	if !b.Config.TelegramAllowlist[item.Message.From.ID] {
		b.logger().WarnContext(ctx, "telegram user denied", "user_id", item.Message.From.ID)
		return
	}
	text := strings.TrimSpace(item.Message.Text)
	fields := strings.Fields(text)
	if len(fields) > 1 && strings.HasPrefix(fields[0], "/") {
		id := fields[1]
		switch fields[0] {
		case "/accept", "/reject":
			status := strings.TrimPrefix(fields[0], "/") + "ed"
			if status == "accepted" || status == "rejected" {
				if err := b.Service.SetStatus(ctx, id, status, ""); err != nil {
					b.send(ctx, item.Message.Chat.ID, "That decision slipped away: "+err.Error())
				} else {
					b.send(ctx, item.Message.Chat.ID, "Done. Dinner has a direction.")
				}
				return
			}
		case "/adjust":
			note := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(text, fields[0]), id))
			if err := b.Service.SetStatus(ctx, id, "adjusted", note); err != nil {
				b.send(ctx, item.Message.Chat.ID, "I couldn't save that adjustment: "+err.Error())
			} else {
				b.send(ctx, item.Message.Chat.ID, "Adjustment noted. I’ll remember the why.")
			}
			return
		case "/shop", "/cook":
			b.sendWorkflow(ctx, item.Message.Chat.ID, id, fields[0] == "/cook")
			return
		}
	}
	decisions, err := b.Service.Plan(ctx, app.PlanInput{Text: text})
	if err != nil {
		b.send(ctx, item.Message.Chat.ID, "Dinner hit a snag: "+err.Error())
		return
	}
	best := decisions[0]
	if len(best.Violations) > 0 {
		b.send(ctx, item.Message.Chat.ID, "I couldn't make every rule work. Closest: "+best.Candidate.Title+"\nNeeds attention: "+strings.Join(best.Violations, "; "))
		return
	}
	message := fmt.Sprintf("%s\n%s\n\n%d min · £%.2f · %d portions · score %.1f\n\n/accept %s\n/reject %s\n/adjust %s <what to change>",
		best.Candidate.Title, best.Candidate.Summary, best.Candidate.TotalMinutes, float64(best.Candidate.CostPence)/100, best.Candidate.Servings, best.Score.Total, best.ID, best.ID, best.ID)
	b.send(ctx, item.Message.Chat.ID, message)
}

func (b Bot) sendWorkflow(ctx context.Context, chatID int64, id string, cooking bool) {
	decisions, err := b.Store.Decisions(ctx, 100)
	if err != nil {
		b.send(ctx, chatID, "Couldn’t load that recipe.")
		return
	}
	for _, decision := range decisions {
		if decision.ID != id {
			continue
		}
		var lines []string
		if cooking {
			lines = append(lines, "Aprons on: "+decision.Candidate.Title)
			for i, step := range decision.Candidate.Steps {
				lines = append(lines, fmt.Sprintf("%d. %s (%d min)\n%s", i+1, step.Title, step.Minutes, step.Detail))
			}
		} else {
			lines = append(lines, "Shopping for "+decision.Candidate.Title)
			for _, item := range app.Shopping(decision.Candidate) {
				lines = append(lines, fmt.Sprintf("□ %g %s %s", item.Quantity, item.Unit, item.Name))
			}
		}
		b.send(ctx, chatID, strings.Join(lines, "\n"))
		return
	}
	b.send(ctx, chatID, "I can’t find that dinner decision.")
}

func (b Bot) send(ctx context.Context, chatID int64, text string) {
	if err := b.call(ctx, "sendMessage", map[string]any{"chat_id": chatID, "text": text}, nil); err != nil {
		b.logger().ErrorContext(ctx, "telegram send failed", "error", err)
	}
}

func (b Bot) call(ctx context.Context, method string, payload any, target any) error {
	body, _ := json.Marshal(payload)
	endpoint := "https://api.telegram.org/bot" + b.Config.TelegramToken + "/" + url.PathEscape(method)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := b.Client
	if client == nil {
		client = &http.Client{Timeout: 40 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return err
	}
	if res.StatusCode/100 != 2 {
		return fmt.Errorf("telegram %s returned %s: %s", method, res.Status, raw)
	}
	if target != nil {
		return json.Unmarshal(raw, target)
	}
	return nil
}

func (b Bot) logger() *slog.Logger {
	if b.Logger != nil {
		return b.Logger
	}
	return slog.Default()
}

func AllowedUserIDs(value string) map[int64]bool {
	out := map[int64]bool{}
	for _, raw := range strings.Split(value, ",") {
		if id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64); err == nil {
			out[id] = true
		}
	}
	return out
}
