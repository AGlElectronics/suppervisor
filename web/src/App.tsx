import { useEffect, useMemo, useState } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import * as Progress from "@radix-ui/react-progress";
import * as Switch from "@radix-ui/react-switch";
import * as Tabs from "@radix-ui/react-tabs";
import {
  BookOpen,
  Check,
  ChefHat,
  ChevronRight,
  CircleGauge,
  Clock3,
  CookingPot,
  Flame,
  Heart,
  History,
  ListChecks,
  LogIn,
  LogOut,
  Moon,
  PackageCheck,
  PoundSterling,
  RotateCcw,
  Settings2,
  ShoppingBasket,
  Sparkles,
  Star,
  Sun,
  UserRound,
  UsersRound,
  WifiOff,
  X,
} from "lucide-react";
import {
  api,
  demoData,
  type Bootstrap,
  type Decision,
  type Rating,
  type WorkflowState,
} from "./api";
import { initialiseAuth, login, logout, type AuthState } from "./auth";
import { brand } from "./brand";

type Screen =
  | "tonight"
  | "recipe"
  | "shopping"
  | "cooking"
  | "profile"
  | "history"
  | "manage";
const navigation = [
  { id: "tonight", label: "Tonight", icon: Sparkles },
  { id: "recipe", label: "Recipe", icon: BookOpen },
  { id: "shopping", label: "Shopping", icon: ShoppingBasket },
  { id: "cooking", label: "Cooking", icon: Flame },
  { id: "profile", label: "Profile", icon: UserRound },
  { id: "history", label: "History", icon: History },
] satisfies { id: Screen; label: string; icon: typeof Sparkles }[];

function readLocalWorkflow(decisionId: string): WorkflowState | null {
  try {
    const value = localStorage.getItem(`suppervisor-workflow:${decisionId}`);
    return value ? (JSON.parse(value) as WorkflowState) : null;
  } catch {
    return null;
  }
}

function App() {
  const [screen, setScreen] = useState<Screen>("tonight");
  const [data, setData] = useState<Bootstrap>(demoData);
  const [offline, setOffline] = useState(false);
  const [dark, setDark] = useState(
    () => localStorage.getItem("suppervisor-theme") === "dark",
  );
  const [brief, setBrief] = useState(
    "Something cosy under 35 minutes, with two lunches",
  );
  const [planning, setPlanning] = useState(false);
  const [notice, setNotice] = useState("");
  const [ratingOpen, setRatingOpen] = useState(false);
  const [adjustOpen, setAdjustOpen] = useState(false);
  const [auth, setAuth] = useState<AuthState>({
    ready: false,
    required: false,
    user: null,
    error: "",
  });

  useEffect(() => {
    document.documentElement.classList.toggle("dark", dark);
    localStorage.setItem("suppervisor-theme", dark ? "dark" : "light");
  }, [dark]);
  useEffect(() => {
    initialiseAuth().then(setAuth);
  }, []);
  useEffect(() => {
    if (!auth.ready || (auth.required && !auth.user)) return;
    api
      .bootstrap()
      .then(setData)
      .catch(() => setOffline(true));
  }, [auth.ready, auth.required, auth.user]);
  const tonight = useMemo(
    () =>
      data.decisions.find((d) => d.status === "accepted") ??
      data.decisions.find((d) => d.status === "proposed") ??
      data.decisions[0],
    [data.decisions],
  );
  const persistedWorkflow = tonight
    ? data.workflowStates.find((state) => state.decisionId === tonight.id)
    : undefined;
  const localWorkflow = tonight ? readLocalWorkflow(tonight.id) : null;
  const workflow = offline && localWorkflow ? localWorkflow : persistedWorkflow;
  const checked = workflow?.shoppingChecked ?? [];
  const step = workflow?.cookingStep ?? 0;

  async function makePlan(event: React.FormEvent) {
    event.preventDefault();
    if (!brief.trim()) return;
    setPlanning(true);
    setNotice("");
    try {
      const result = await api.plan(brief);
      setData((current) => ({
        ...current,
        decisions: [...result.decisions, ...current.decisions],
      }));
      setOffline(false);
    } catch (error) {
      setNotice(error instanceof Error ? error.message : brand.voice.error);
    } finally {
      setPlanning(false);
    }
  }
  async function updateDecision(status: Decision["status"], correction = "") {
    if (!tonight) return;
    try {
      if (!offline) await api.setDecision(tonight.id, status, correction);
      setData((current) => ({
        ...current,
        decisions: current.decisions.map((d) =>
          d.id === tonight.id ? { ...d, status, correction } : d,
        ),
        decisionEvents:
          tonight.status === status && tonight.correction === correction
            ? current.decisionEvents
            : [
                {
                  id: crypto.randomUUID(),
                  decisionId: tonight.id,
                  type: status,
                  actor: "user",
                  detail: { correction },
                  createdAt: new Date().toISOString(),
                },
                ...current.decisionEvents,
              ],
      }));
      setNotice(
        status === "accepted"
          ? brand.voice.accepted
          : "Noted. Better options next time.",
      );
    } catch (error) {
      setNotice(error instanceof Error ? error.message : brand.voice.error);
    }
  }

  async function updateWorkflow(
    shoppingChecked: string[],
    cookingStep: number,
  ) {
    if (!tonight) return;
    const optimistic = {
      decisionId: tonight.id,
      shoppingChecked,
      cookingStep,
      updatedAt: new Date().toISOString(),
    };
    localStorage.setItem(
      `suppervisor-workflow:${tonight.id}`,
      JSON.stringify(optimistic),
    );
    setData((current) => ({
      ...current,
      workflowStates: [
        optimistic,
        ...current.workflowStates.filter(
          (state) => state.decisionId !== tonight.id,
        ),
      ],
    }));
    if (offline) return;
    try {
      const saved = await api.setWorkflow(
        tonight.id,
        shoppingChecked,
        cookingStep,
      );
      setData((current) => ({
        ...current,
        workflowStates: [
          saved,
          ...current.workflowStates.filter(
            (state) => state.decisionId !== tonight.id,
          ),
        ],
      }));
    } catch (error) {
      setNotice(
        error instanceof Error ? error.message : "Couldn’t save progress",
      );
    }
  }

  if (!auth.ready) return <AuthLoading />;
  if (auth.required && !auth.user) {
    return <LoginScreen error={auth.error} onLogin={() => login()} />;
  }

  return (
    <div className="min-h-dvh bg-canvas text-ink">
      <a className="skip-link" href="#main-content">
        Skip to content
      </a>
      <aside className="sidebar">
        <BrandMark />
        <nav aria-label="Main navigation" className="mt-8 grid gap-1">
          {navigation.map(({ id, label, icon: Icon }) => (
            <button
              key={id}
              className="nav-item"
              aria-current={screen === id ? "page" : undefined}
              onClick={() => setScreen(id)}
            >
              <Icon size={19} />
              <span>{label}</span>
            </button>
          ))}
        </nav>
        <div className="mt-auto space-y-2 pt-8">
          <button
            className="nav-item"
            aria-current={screen === "manage" ? "page" : undefined}
            onClick={() => setScreen("manage")}
          >
            <Settings2 size={19} />
            <span>Manage</span>
          </button>
          {auth.user && (
            <button className="nav-item" onClick={() => logout()}>
              <LogOut size={19} />
              <span>Sign out</span>
            </button>
          )}
          <div className="theme-row">
            <Sun size={16} />
            <span>Kitchen light</span>
            <Switch.Root
              className="switch"
              checked={dark}
              onCheckedChange={setDark}
              aria-label="Use dark theme"
            >
              <Switch.Thumb className="switch-thumb" />
            </Switch.Root>
            <Moon size={16} />
          </div>
        </div>
      </aside>
      <header className="mobile-header">
        <BrandMark compact />
        <div className="mobile-actions">
          <button
            className="icon-button"
            onClick={() => setScreen("manage")}
            aria-label="Open management"
          >
            <Settings2 size={20} />
          </button>
          <button
            className="icon-button"
            onClick={() => setDark((value) => !value)}
            aria-label="Toggle colour theme"
          >
            {dark ? <Sun size={20} /> : <Moon size={20} />}
          </button>
          {auth.user && (
            <button
              className="icon-button"
              onClick={() => logout()}
              aria-label="Sign out"
            >
              <LogOut size={20} />
            </button>
          )}
        </div>
      </header>
      <main className="main-shell" id="main-content" tabIndex={-1}>
        {offline && (
          <div className="demo-banner" role="status">
            <WifiOff size={17} />
            Previewing seeded demo data. Connect the API to make changes stick.
          </div>
        )}
        {notice && (
          <div className="notice" role="status">
            {notice}
            <button onClick={() => setNotice("")} aria-label="Dismiss">
              <X size={16} />
            </button>
          </div>
        )}
        {screen === "tonight" && (
          <Tonight
            decision={tonight}
            brief={brief}
            setBrief={setBrief}
            planning={planning}
            onPlan={makePlan}
            onAccept={() => updateDecision("accepted")}
            onReject={() => updateDecision("rejected")}
            onAdjust={() => setAdjustOpen(true)}
            onOpen={() => setScreen("recipe")}
          />
        )}
        {screen === "recipe" && (
          <Recipe
            decision={tonight}
            onShop={() => setScreen("shopping")}
            onCook={() => setScreen("cooking")}
          />
        )}
        {screen === "shopping" && (
          <Shopping
            decision={tonight}
            checked={checked}
            onToggle={(name) => {
              const next = checked.includes(name)
                ? checked.filter((item) => item !== name)
                : [...checked, name];
              void updateWorkflow(next, step);
            }}
          />
        )}
        {screen === "cooking" && (
          <Cooking
            decision={tonight}
            step={step}
            setStep={(next) => {
              void updateWorkflow(checked, next);
            }}
            onFinish={() => {
              updateDecision("cooked");
              setRatingOpen(true);
            }}
          />
        )}
        {screen === "profile" && <Profiles data={data} />}
        {screen === "history" && <HistoryView decisions={data.decisions} />}
        {screen === "manage" && (
          <Management
            data={data}
            offline={offline}
            onRevert={async (id) => {
              if (!offline) {
                await api.revertPreference(id);
                setData(await api.bootstrap());
                return;
              }
              setData((current) => ({
                ...current,
                preferenceEvents: current.preferenceEvents.map((event) =>
                  event.id === id
                    ? { ...event, revertedAt: new Date().toISOString() }
                    : event,
                ),
              }));
            }}
          />
        )}
      </main>
      <nav className="mobile-nav" aria-label="Mobile navigation">
        {navigation.map(({ id, label, icon: Icon }) => (
          <button
            key={id}
            aria-current={screen === id ? "page" : undefined}
            onClick={() => setScreen(id)}
          >
            <Icon size={20} />
            <span>{label}</span>
          </button>
        ))}
      </nav>
      {tonight && (
        <>
          <AdjustmentDialog
            open={adjustOpen}
            onOpenChange={setAdjustOpen}
            onSave={(correction) => updateDecision("adjusted", correction)}
          />
          <RatingDialog
            open={ratingOpen}
            onOpenChange={setRatingOpen}
            decision={tonight}
            profiles={data.profiles}
            offline={offline}
            onSaved={(events, rating) =>
              setData((current) => ({
                ...current,
                preferenceEvents: [...events, ...current.preferenceEvents],
                ratings: [
                  rating,
                  ...current.ratings.filter(
                    (item) =>
                      item.decisionId !== rating.decisionId ||
                      item.profileId !== rating.profileId,
                  ),
                ],
              }))
            }
          />
        </>
      )}
    </div>
  );
}

function AuthLoading() {
  return (
    <main className="auth-shell" aria-busy="true">
      <BrandMark />
      <p>Setting the table…</p>
    </main>
  );
}

function LoginScreen({
  error,
  onLogin,
}: {
  error: string;
  onLogin: () => Promise<void>;
}) {
  const [busy, setBusy] = useState(false);
  const [loginError, setLoginError] = useState(error);
  return (
    <main className="auth-shell">
      <BrandMark />
      <div>
        <span className="eyebrow">Private kitchen</span>
        <h1>Come in. Dinner’s waiting.</h1>
        <p>Sign in through the household Keycloak to open Suppervisor.</p>
      </div>
      {(loginError || error) && (
        <p className="warning" role="alert">
          {loginError || error}
        </p>
      )}
      <button
        className="primary-button"
        disabled={busy}
        onClick={async () => {
          setBusy(true);
          setLoginError("");
          try {
            await onLogin();
          } catch (loginFailure) {
            setLoginError(
              loginFailure instanceof Error
                ? loginFailure.message
                : "Sign-in could not start",
            );
            setBusy(false);
          }
        }}
      >
        <LogIn size={18} />
        {busy ? "Opening Keycloak…" : "Sign in"}
      </button>
    </main>
  );
}

function AdjustmentDialog({
  open,
  onOpenChange,
  onSave,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSave: (correction: string) => void;
}) {
  const [correction, setCorrection] = useState("");
  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="dialog-overlay" />
        <Dialog.Content className="dialog-content">
          <Dialog.Title>What should change?</Dialog.Title>
          <Dialog.Description>
            Keep the useful parts and leave a correction we can inspect later.
          </Dialog.Description>
          <label>
            Adjustment
            <textarea
              autoFocus
              value={correction}
              onChange={(event) => setCorrection(event.target.value)}
              placeholder="Less spicy, or swap the oven for the hob…"
            />
          </label>
          <div className="flex justify-end gap-2">
            <Dialog.Close className="secondary-button">Cancel</Dialog.Close>
            <button
              className="primary-button"
              disabled={!correction.trim()}
              onClick={() => {
                onSave(correction.trim());
                setCorrection("");
                onOpenChange(false);
              }}
            >
              Save adjustment
            </button>
          </div>
          <Dialog.Close className="dialog-x" aria-label="Close">
            <X />
          </Dialog.Close>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

function BrandMark({ compact = false }: { compact?: boolean }) {
  return (
    <div className="flex items-center gap-3">
      <span className="brand-icon">
        <ChefHat size={compact ? 20 : 24} />
      </span>
      <span>
        <strong className="font-display text-lg">{brand.name}</strong>
        {!compact && (
          <small className="block text-xs text-muted">{brand.tagline}</small>
        )}
      </span>
    </div>
  );
}
function PageIntro({
  eyebrow,
  title,
  children,
}: {
  eyebrow: string;
  title: string;
  children?: React.ReactNode;
}) {
  return (
    <header className="page-intro">
      <span className="eyebrow">{eyebrow}</span>
      <h1>{title}</h1>
      {children && <p>{children}</p>}
    </header>
  );
}
function Tonight({
  decision,
  brief,
  setBrief,
  planning,
  onPlan,
  onAccept,
  onReject,
  onAdjust,
  onOpen,
}: {
  decision?: Decision;
  brief: string;
  setBrief: (v: string) => void;
  planning: boolean;
  onPlan: (e: React.FormEvent) => void;
  onAccept: () => void;
  onReject: () => void;
  onAdjust: () => void;
  onOpen: () => void;
}) {
  return (
    <div className="page-grid">
      <section>
        <PageIntro eyebrow="Tonight" title="What should dinner do for you?">
          Say it naturally. Suppervisor turns the brief into rules, then ranks
          what actually fits.
        </PageIntro>
        <form className="brief-box" onSubmit={onPlan}>
          <label htmlFor="dinner-brief">Dinner brief</label>
          <textarea
            id="dinner-brief"
            value={brief}
            onChange={(e) => setBrief(e.target.value)}
            rows={3}
            placeholder="Quick, comforting, no mushrooms, and enough for lunch…"
          />
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex gap-2 text-xs text-muted">
              <span className="chip">Alan + Partner</span>
              <span className="chip">Natural language</span>
            </div>
            <button className="primary-button" disabled={planning}>
              {planning ? "Sizing up supper…" : "Find dinner"}
              <ChevronRight size={18} />
            </button>
          </div>
        </form>
      </section>
      {decision ? (
        <CandidateCard
          decision={decision}
          onAccept={onAccept}
          onReject={onReject}
          onAdjust={onAdjust}
          onOpen={onOpen}
        />
      ) : (
        <Empty />
      )}
    </div>
  );
}
function CandidateCard({
  decision,
  onAccept,
  onReject,
  onAdjust,
  onOpen,
}: {
  decision: Decision;
  onAccept: () => void;
  onReject: () => void;
  onAdjust: () => void;
  onOpen: () => void;
}) {
  const recipe = decision.candidate;
  return (
    <article className="hero-card">
      <div className="hero-card-top">
        <span className="score-ring">
          {decision.score.total}
          <small>fit</small>
        </span>
        <div>
          <span className="eyebrow">
            {decision.status === "accepted" ? "Tonight’s plan" : "Top match"}
          </span>
          <h2>{recipe.title}</h2>
        </div>
      </div>
      <p className="mt-4 text-muted">{recipe.summary}</p>
      <div className="metric-row">
        <span>
          <Clock3 />
          {recipe.totalMinutes} min
        </span>
        <span>
          <PoundSterling />
          {(recipe.costPence / 100).toFixed(2)}
        </span>
        <span>
          <UsersRound />
          {recipe.servings} portions
        </span>
        <span>
          <PackageCheck />
          {recipe.reheatsWell ? "Great reheated" : "Best eaten fresh"}
        </span>
      </div>
      {decision.violations.length > 0 && (
        <div className="warning">
          <strong>Needs attention</strong>
          {decision.violations.join(" · ")}
        </div>
      )}
      <details className="score-details">
        <summary>Why this fits</summary>
        <dl>
          {Object.entries(decision.score)
            .filter(([key]) => key !== "total")
            .map(([key, value]) => (
              <div key={key}>
                <dt>{key}</dt>
                <dd>{value}</dd>
              </div>
            ))}
        </dl>
        {(decision.request.assumptions?.length ?? 0) > 0 && (
          <p>
            Assumed: {decision.request.assumptions?.join(" · ")}. Adjust if
            that’s wrong.
          </p>
        )}
      </details>
      <div className="mt-6 flex flex-wrap gap-2">
        <button className="primary-button" onClick={onAccept}>
          <Check size={18} />
          This one
        </button>
        <button className="secondary-button" onClick={onOpen}>
          See recipe
        </button>
        <button className="ghost-button" onClick={onAdjust}>
          Adjust
        </button>
        <button className="ghost-button" onClick={onReject}>
          <X size={17} />
          Not tonight
        </button>
      </div>
    </article>
  );
}
function Recipe({
  decision,
  onShop,
  onCook,
}: {
  decision?: Decision;
  onShop: () => void;
  onCook: () => void;
}) {
  if (!decision) return <Empty />;
  const { candidate } = decision;
  return (
    <div>
      <PageIntro eyebrow="Recipe" title={candidate.title}>
        {candidate.summary}
      </PageIntro>
      <div className="content-grid">
        <section className="panel">
          <h2>What you’ll need</h2>
          <ul className="ingredient-list">
            {candidate.ingredients.map((item) => (
              <li key={item.name}>
                <span>{item.name}</span>
                <strong>
                  {item.quantity} {item.unit}
                </strong>
              </li>
            ))}
          </ul>
          <button className="secondary-button mt-5" onClick={onShop}>
            <ShoppingBasket size={18} />
            Open shopping
          </button>
        </section>
        <section className="panel prep-panel">
          <span className="eyebrow">Prep first</span>
          <h2>Five calm minutes before the heat</h2>
          <ul className="prep-list">
            {candidate.ingredients.slice(0, 4).map((item) => (
              <li key={item.name}>
                <Check size={16} />
                <span>
                  Measure {item.quantity} {item.unit} {item.name}; drain, rinse,
                  or chop as needed.
                </span>
              </li>
            ))}
            {decision.request.lunchPortions > 0 && (
              <li>
                <PackageCheck size={16} />
                <span>
                  Set out containers for {decision.request.lunchPortions} lunch
                  portions.
                </span>
              </li>
            )}
          </ul>
        </section>
        <section className="panel">
          <h2>The game plan</h2>
          <ol className="step-list">
            {candidate.steps.map((item, index) => (
              <li key={item.title}>
                <span>{index + 1}</span>
                <div>
                  <strong>{item.title}</strong>
                  <p>{item.detail}</p>
                </div>
                <small>{item.minutes} min</small>
              </li>
            ))}
          </ol>
          <button className="primary-button mt-5" onClick={onCook}>
            <Flame size={18} />
            Start cooking
          </button>
        </section>
      </div>
    </div>
  );
}
function Shopping({
  decision,
  checked,
  onToggle,
}: {
  decision?: Decision;
  checked: string[];
  onToggle: (name: string) => void;
}) {
  if (!decision) return <Empty />;
  const pantry = new Set(
    (decision.request.available ?? []).map((item) => item.trim().toLowerCase()),
  );
  const items = decision.candidate.ingredients.filter(
    (item) => !pantry.has(item.name.trim().toLowerCase()),
  );
  const complete = items.length ? (checked.length / items.length) * 100 : 0;
  return (
    <div>
      <PageIntro eyebrow="Shopping" title="A short, useful list">
        {checked.length} of {items.length} tucked into the basket.
      </PageIntro>
      <Progress.Root className="progress-track" value={complete}>
        <Progress.Indicator
          className="progress-fill"
          style={{ transform: `translateX(-${100 - complete}%)` }}
        />
      </Progress.Root>
      <div className="panel mt-5">
        <ul className="shopping-list">
          {items.map((item) => (
            <li key={item.name}>
              <label>
                <input
                  type="checkbox"
                  checked={checked.includes(item.name)}
                  onChange={() => onToggle(item.name)}
                />
                <span className="checkmark">
                  <Check size={15} />
                </span>
                <span
                  className={
                    checked.includes(item.name) ? "line-through text-muted" : ""
                  }
                >
                  {item.name}
                </span>
                <strong>
                  {item.quantity} {item.unit}
                </strong>
              </label>
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}
function Cooking({
  decision,
  step,
  setStep,
  onFinish,
}: {
  decision?: Decision;
  step: number;
  setStep: (v: number) => void;
  onFinish: () => void;
}) {
  if (!decision) return <Empty />;
  const steps = decision.candidate.steps;
  const current = steps[Math.min(step, steps.length - 1)];
  return (
    <div className="cooking-shell">
      <PageIntro eyebrow="Cooking" title={brand.voice.cooking}>
        {decision.candidate.title}
      </PageIntro>
      <div className="cook-progress">
        {steps.map((item, index) => (
          <button
            key={item.title}
            className={index <= step ? "active" : ""}
            onClick={() => setStep(index)}
            aria-label={`Go to step ${index + 1}`}
          >
            {index < step ? <Check size={15} /> : index + 1}
          </button>
        ))}
      </div>
      <article className="cook-card">
        <span className="eyebrow">
          Step {step + 1} of {steps.length} · {current.minutes} minutes
        </span>
        <h2>{current.title}</h2>
        <p>{current.detail}</p>
        <div className="mt-8 flex justify-between gap-3">
          <button
            className="secondary-button"
            disabled={step === 0}
            onClick={() => setStep(Math.max(0, step - 1))}
          >
            Back
          </button>
          {step < steps.length - 1 ? (
            <button
              className="primary-button"
              onClick={() => setStep(step + 1)}
            >
              Next step
              <ChevronRight size={18} />
            </button>
          ) : (
            <button className="primary-button" onClick={onFinish}>
              <Check size={18} />
              Dinner done
            </button>
          )}
        </div>
      </article>
    </div>
  );
}
function Profiles({ data }: { data: Bootstrap }) {
  return (
    <div>
      <PageIntro eyebrow="Profiles" title="Different people, separate tastes">
        Hard dislikes stay hard. Softer preferences can learn from every rating.
      </PageIntro>
      <div className="card-grid">
        {data.profiles.map((profile) => (
          <article className="panel" key={profile.id}>
            <div className="profile-heading">
              <span>{profile.name[0]}</span>
              <div>
                <h2>{profile.name}</h2>
                <small>Individual dinner profile</small>
              </div>
            </div>
            <h3>Known winners</h3>
            <div className="tag-row">
              {profile.likes.map((item) => (
                <span className="positive-tag" key={item}>
                  <Heart size={13} />
                  {item}
                </span>
              ))}
            </div>
            <h3>Hard no, thanks</h3>
            <div className="tag-row">
              {profile.dislikes.map((item) => (
                <span className="negative-tag" key={item}>
                  {item}
                </span>
              ))}
            </div>
            <h3>Learned leanings</h3>
            {Object.entries(profile.preferences).map(([key, value]) => (
              <div className="preference-bar" key={key}>
                <span>{key}</span>
                <div>
                  <i style={{ width: `${Math.min(100, 45 + value * 25)}%` }} />
                </div>
                <strong>
                  {value > 0 ? "+" : ""}
                  {value.toFixed(2)}
                </strong>
              </div>
            ))}
          </article>
        ))}
      </div>
    </div>
  );
}
function HistoryView({ decisions }: { decisions: Decision[] }) {
  return (
    <div>
      <PageIntro eyebrow="History" title="Dinner’s paper trail">
        What won, what didn’t, and why. No mysterious recommendations.
      </PageIntro>
      <div className="timeline">
        {decisions.map((d) => (
          <article key={d.id}>
            <span className={`status-dot ${d.status}`} />
            <div>
              <small>
                {new Date(d.createdAt).toLocaleDateString()} · {d.status}
              </small>
              <h2>{d.candidate.title}</h2>
              <p>{d.request.text}</p>
              {d.correction && (
                <p className="correction">Correction: {d.correction}</p>
              )}
            </div>
            <strong>{d.score.total}</strong>
          </article>
        ))}
      </div>
    </div>
  );
}
function Management({
  data,
  offline,
  onRevert,
}: {
  data: Bootstrap;
  offline: boolean;
  onRevert: (id: string) => void;
}) {
  const tabs = [
    "profiles",
    "preferences",
    "meals",
    "ratings",
    "recipes",
    "decisions",
    "status",
    "corrections",
  ];
  const corrections = data.decisionEvents.filter(
    (event) =>
      event.type === "adjusted" &&
      typeof event.detail.correction === "string" &&
      event.detail.correction,
  );
  return (
    <div>
      <PageIntro eyebrow="Management" title="Everything with the lid lifted">
        Profiles, recipes, decisions and learning — inspectable and correctable.
      </PageIntro>
      <Tabs.Root defaultValue="profiles" className="manage-tabs">
        <Tabs.List aria-label="Management sections">
          {tabs.map((tab) => (
            <Tabs.Trigger key={tab} value={tab}>
              {tab}
            </Tabs.Trigger>
          ))}
        </Tabs.List>
        <Tabs.Content value="profiles">
          <DataTable
            rows={data.profiles.map((p) => [
              p.name,
              p.likes.join(", "),
              p.dislikes.join(", "),
            ])}
            headings={["Profile", "Likes", "Dislikes"]}
          />
        </Tabs.Content>
        <Tabs.Content value="preferences">
          <div className="stack-list">
            {data.preferenceEvents.length ? (
              data.preferenceEvents.map((event) => (
                <article key={event.id}>
                  <div>
                    <strong>
                      {event.profileId} · {event.key}
                    </strong>
                    <p>{event.reason}</p>
                  </div>
                  <span className={event.delta >= 0 ? "good" : "bad"}>
                    {event.delta > 0 ? "+" : ""}
                    {event.delta}
                  </span>
                  <button
                    className="icon-button"
                    disabled={!!event.revertedAt || !!event.reversalOf}
                    onClick={() => onRevert(event.id)}
                    aria-label={`Revert ${event.key} preference`}
                  >
                    <RotateCcw size={17} />
                  </button>
                </article>
              ))
            ) : (
              <Empty compact />
            )}
          </div>
        </Tabs.Content>
        <Tabs.Content value="meals">
          <DataTable
            rows={data.decisions
              .filter((d) => ["accepted", "cooked"].includes(d.status))
              .map((d) => [
                d.candidate.title,
                d.status,
                new Date(d.createdAt).toLocaleDateString(),
              ])}
            headings={["Meal", "State", "Date"]}
          />
        </Tabs.Content>
        <Tabs.Content value="ratings">
          {data.ratings?.length ? (
            <DataTable
              rows={data.ratings.map((r) => [
                r.profileId,
                `${r.overall}/5`,
                `${r.taste}/5`,
                `${r.ease}/5`,
                `${r.reheat}/5`,
                r.notes,
              ])}
              headings={[
                "Profile",
                "Overall",
                "Taste",
                "Ease",
                "Reheat",
                "Notes",
              ]}
            />
          ) : (
            <Empty compact />
          )}
        </Tabs.Content>
        <Tabs.Content value="recipes">
          <DataTable
            rows={data.recipes.map((r) => [
              r.title,
              `${r.totalMinutes} min`,
              `£${(r.costPence / 100).toFixed(2)}`,
            ])}
            headings={["Recipe", "Time", "Cost"]}
          />
        </Tabs.Content>
        <Tabs.Content value="decisions">
          <DataTable
            rows={data.decisions.map((d) => [
              d.candidate.title,
              d.status,
              `${d.score.total} fit`,
            ])}
            headings={["Decision", "State", "Score"]}
          />
        </Tabs.Content>
        <Tabs.Content value="status">
          <div className="status-grid">
            <StatusCard
              icon={CircleGauge}
              title="API"
              value={offline ? "Demo mode" : "Connected"}
            />
            <StatusCard
              icon={CookingPot}
              title="Generation"
              value={offline ? "Seeded fallback" : "Ollama via server"}
            />
            <StatusCard icon={ListChecks} title="Rules" value="Deterministic" />
          </div>
        </Tabs.Content>
        <Tabs.Content value="corrections">
          <div className="stack-list">
            {corrections.map((event) => (
              <article key={event.id}>
                <div>
                  <strong>
                    {data.decisions.find(
                      (decision) => decision.id === event.decisionId,
                    )?.candidate.title ?? "Dinner decision"}
                  </strong>
                  <p>{String(event.detail.correction)}</p>
                </div>
                <span className="chip">adjusted</span>
              </article>
            ))}
            {corrections.length === 0 && <Empty compact />}
          </div>
        </Tabs.Content>
      </Tabs.Root>
    </div>
  );
}
function DataTable({
  headings,
  rows,
}: {
  headings: string[];
  rows: string[][];
}) {
  return (
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            {headings.map((h) => (
              <th key={h}>{h}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, i) => (
            <tr key={i}>
              {row.map((cell, j) => (
                <td key={j}>{cell}</td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
      {rows.length === 0 && <Empty compact />}
    </div>
  );
}
function StatusCard({
  icon: Icon,
  title,
  value,
}: {
  icon: typeof CircleGauge;
  title: string;
  value: string;
}) {
  return (
    <article className="panel">
      <Icon size={22} />
      <small>{title}</small>
      <strong>{value}</strong>
    </article>
  );
}
function RatingDialog({
  open,
  onOpenChange,
  decision,
  profiles,
  offline,
  onSaved,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  decision: Decision;
  profiles: Bootstrap["profiles"];
  offline: boolean;
  onSaved: (events: Bootstrap["preferenceEvents"], rating: Rating) => void;
}) {
  const [profileId, setProfileId] = useState(profiles[0]?.id ?? "alan");
  const [overall, setOverall] = useState(4);
  const [taste, setTaste] = useState(4);
  const [ease, setEase] = useState(4);
  const [reheat, setReheat] = useState(4);
  const [tags, setTags] = useState<string[]>([]);
  const [notes, setNotes] = useState("");
  async function save() {
    const rating: Rating = {
      decisionId: decision.id,
      profileId,
      overall,
      taste,
      ease,
      reheat,
      tags,
      notes,
    };
    let events: Bootstrap["preferenceEvents"] = [];
    if (!offline) {
      const result = await api.rate(rating);
      events = result.preferenceEvents;
    }
    onSaved(events, rating);
    onOpenChange(false);
  }
  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="dialog-overlay" />
        <Dialog.Content className="dialog-content">
          <Dialog.Title>How did dinner land?</Dialog.Title>
          <Dialog.Description>
            Rate your own plate. Alan and Partner never share a score.
          </Dialog.Description>
          <label>
            Who’s rating?
            <select
              value={profileId}
              onChange={(e) => setProfileId(e.target.value)}
            >
              {profiles.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
            </select>
          </label>
          <div className="rating-grid">
            <ScoreInput label="Overall" value={overall} onChange={setOverall} />
            <ScoreInput label="Taste" value={taste} onChange={setTaste} />
            <ScoreInput label="Ease" value={ease} onChange={setEase} />
            <ScoreInput label="Reheating" value={reheat} onChange={setReheat} />
          </div>
          <fieldset>
            <legend>Quick notes</legend>
            <div className="tag-row">
              {["make again", "too fussy", "great lunch", "needs crunch"].map(
                (tag) => (
                  <button
                    type="button"
                    className={tags.includes(tag) ? "positive-tag" : "chip"}
                    aria-pressed={tags.includes(tag)}
                    key={tag}
                    onClick={() =>
                      setTags((current) =>
                        current.includes(tag)
                          ? current.filter((item) => item !== tag)
                          : [...current, tag],
                      )
                    }
                  >
                    {tag}
                  </button>
                ),
              )}
            </div>
          </fieldset>
          <label>
            Anything worth remembering?
            <textarea
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              placeholder="The crispy bits disappeared first…"
            />
          </label>
          <div className="flex justify-end gap-2">
            <Dialog.Close className="secondary-button">Later</Dialog.Close>
            <button className="primary-button" onClick={save}>
              Save rating
            </button>
          </div>
          <Dialog.Close className="dialog-x" aria-label="Close">
            <X />
          </Dialog.Close>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

function ScoreInput({
  label,
  value,
  onChange,
}: {
  label: string;
  value: number;
  onChange: (value: number) => void;
}) {
  return (
    <fieldset>
      <legend>{label}</legend>
      <div className="star-row">
        {[1, 2, 3, 4, 5].map((score) => (
          <button
            type="button"
            key={score}
            onClick={() => onChange(score)}
            aria-label={`${label}: ${score} out of 5`}
            aria-pressed={score === value}
          >
            <Star fill={score <= value ? "currentColor" : "none"} />
          </button>
        ))}
      </div>
    </fieldset>
  );
}
function Empty({ compact = false }: { compact?: boolean }) {
  return (
    <div className={compact ? "empty-compact" : "empty-card"}>
      <CookingPot size={compact ? 22 : 32} />
      <p>{brand.voice.emptyTonight}</p>
    </div>
  );
}

export default App;
