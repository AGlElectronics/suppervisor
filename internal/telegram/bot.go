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
	"github.com/AGlElectronics/suppervisor/internal/domain"
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
		if err := b.handle(r.Context(), item); err != nil {
			http.Error(w, "update processing failed", http.StatusInternalServerError)
			return
		}
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
			if err := b.handle(ctx, item); err != nil {
				break
			}
			offset = item.UpdateID + 1
		}
	}
	return ctx.Err()
}

func (b Bot) handle(ctx context.Context, item update) error {
	if item.Message == nil {
		return nil
	}
	claimed, err := b.Store.ClaimTelegramUpdate(ctx, item.UpdateID)
	if err != nil {
		b.logger().ErrorContext(ctx, "telegram update claim failed", "update_id", item.UpdateID, "error", err)
		return err
	}
	if !claimed {
		b.logger().InfoContext(ctx, "telegram duplicate ignored", "update_id", item.UpdateID)
		return nil
	}
	if err := b.process(ctx, item); err != nil {
		if markErr := b.Store.FailTelegramUpdate(ctx, item.UpdateID, err); markErr != nil {
			b.logger().ErrorContext(ctx, "telegram update failure mark failed", "update_id", item.UpdateID, "error", markErr)
		}
		b.logger().ErrorContext(ctx, "telegram update processing failed", "update_id", item.UpdateID, "error", err)
		return err
	}
	if err := b.Store.CompleteTelegramUpdate(ctx, item.UpdateID); err != nil {
		b.logger().ErrorContext(ctx, "telegram update completion failed", "update_id", item.UpdateID, "error", err)
		return err
	}
	return nil
}

func (b Bot) process(ctx context.Context, item update) error {
	if !b.Config.TelegramAllowlist[item.Message.From.ID] {
		b.logger().WarnContext(ctx, "telegram user denied", "user_id", item.Message.From.ID)
		return nil
	}
	text := strings.TrimSpace(item.Message.Text)
	fields := strings.Fields(text)
	if len(fields) > 0 && strings.HasPrefix(fields[0], "/") {
		command := strings.SplitN(fields[0], "@", 2)[0]
		if command == "/start" || command == "/help" {
			return b.send(ctx, item.Message.Chat.ID, "Tell me what dinner needs to do, naturally.\n\nCommands after I suggest one:\n/accept <decision>\n/reject <decision>\n/adjust <decision> <change>\n/shop <decision>\n/cook <decision>\n/rate <decision> <alan|partner> <overall> <taste> <ease> <reheat> [notes]")
		}
		if command == "/rate" {
			if len(fields) < 7 {
				return b.send(ctx, item.Message.Chat.ID, "Use /rate <decision> <alan|partner> <overall> <taste> <ease> <reheat> [notes]. Scores run from 1 to 5.")
			}
			scores := make([]int, 4)
			for i := range scores {
				scores[i], _ = strconv.Atoi(fields[i+3])
			}
			rating := domain.Rating{
				DecisionID: fields[1], ProfileID: strings.ToLower(fields[2]),
				Overall: scores[0], Taste: scores[1], Ease: scores[2], Reheat: scores[3],
				Notes: strings.Join(fields[7:], " "),
			}
			if rating.ProfileID != "alan" && rating.ProfileID != "partner" {
				return b.send(ctx, item.Message.Chat.ID, "Choose alan or partner so the right profile learns.")
			}
			if _, err := b.Service.Rate(ctx, rating); err != nil {
				return b.send(ctx, item.Message.Chat.ID, "I couldn’t save that rating: "+err.Error())
			} else {
				return b.send(ctx, item.Message.Chat.ID, "Rating saved to "+rating.ProfileID+" — useful, not personal.")
			}
		}
		if len(fields) < 2 {
			return b.send(ctx, item.Message.Chat.ID, "That command needs a decision ID. /help shows the shape.")
		}
		id := fields[1]
		switch command {
		case "/accept", "/reject":
			status := strings.TrimPrefix(command, "/") + "ed"
			if status == "accepted" || status == "rejected" {
				if err := b.Service.SetStatus(ctx, id, status, ""); err != nil {
					return b.send(ctx, item.Message.Chat.ID, "That decision slipped away: "+err.Error())
				} else {
					return b.send(ctx, item.Message.Chat.ID, "Done. Dinner has a direction.")
				}
			}
		case "/adjust":
			note := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(text, fields[0]), id))
			if note == "" {
				return b.send(ctx, item.Message.Chat.ID, "Tell me what should change after the decision ID.")
			}
			replacements, err := b.Service.Adjust(ctx, id, note)
			if err != nil {
				return b.send(ctx, item.Message.Chat.ID, "I couldn't save that adjustment: "+err.Error())
			} else {
				best := replacements[0]
				return b.send(ctx, item.Message.Chat.ID, fmt.Sprintf(
					"Adjustment applied. New top match: %s (score %.1f)\n/accept %s",
					best.Candidate.Title, best.Score.Total, best.ID,
				))
			}
		case "/shop", "/cook":
			return b.sendWorkflow(ctx, item.Message.Chat.ID, id, command == "/cook")
		default:
			return b.send(ctx, item.Message.Chat.ID, "I don’t know that command yet. Try /help, or just describe dinner.")
		}
	}
	decisions, err := b.Service.Plan(ctx, app.PlanInput{Text: text})
	if err != nil {
		return b.send(ctx, item.Message.Chat.ID, "Dinner hit a snag: "+err.Error())
	}
	best := decisions[0]
	if len(best.Violations) > 0 {
		return b.send(ctx, item.Message.Chat.ID, "I couldn't make every rule work. Closest: "+best.Candidate.Title+"\nNeeds attention: "+strings.Join(best.Violations, "; "))
	}
	message := fmt.Sprintf("%s\n%s\n\n%d min · £%.2f · %d portions · score %.1f\n\n/accept %s\n/reject %s\n/adjust %s <what to change>",
		best.Candidate.Title, best.Candidate.Summary, best.Candidate.TotalMinutes, float64(best.Candidate.CostPence)/100, best.Candidate.Servings, best.Score.Total, best.ID, best.ID, best.ID)
	return b.send(ctx, item.Message.Chat.ID, message)
}

func (b Bot) sendWorkflow(ctx context.Context, chatID int64, id string, cooking bool) error {
	decision, err := b.Store.DecisionByID(ctx, id)
	if err != nil {
		return b.send(ctx, chatID, "Couldn’t load that recipe.")
	}
	var lines []string
	if cooking {
		lines = append(lines, "Aprons on: "+decision.Candidate.Title)
		for i, step := range decision.Candidate.Steps {
			lines = append(lines, fmt.Sprintf("%d. %s (%d min)\n%s", i+1, step.Title, step.Minutes, step.Detail))
		}
	} else {
		lines = append(lines, "Shopping for "+decision.Candidate.Title)
		for _, item := range app.Shopping(decision.Candidate, decision.Request.Available) {
			lines = append(lines, fmt.Sprintf("□ %g %s %s", item.Quantity, item.Unit, item.Name))
		}
	}
	return b.send(ctx, chatID, strings.Join(lines, "\n"))
}

func (b Bot) send(ctx context.Context, chatID int64, text string) error {
	for _, chunk := range splitMessage(text, 3900) {
		if err := b.call(ctx, "sendMessage", map[string]any{"chat_id": chatID, "text": chunk}, nil); err != nil {
			b.logger().ErrorContext(ctx, "telegram send failed", "error", err)
			return err
		}
	}
	return nil
}

func splitMessage(text string, limit int) []string {
	runes := []rune(text)
	var chunks []string
	for len(runes) > limit {
		cut := limit
		for cut > limit/2 && runes[cut] != '\n' {
			cut--
		}
		if cut <= limit/2 {
			cut = limit
		}
		chunks = append(chunks, strings.TrimSpace(string(runes[:cut])))
		runes = runes[cut:]
	}
	if tail := strings.TrimSpace(string(runes)); tail != "" {
		chunks = append(chunks, tail)
	}
	return chunks
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
