import { useEffect, useMemo, useState } from 'react'
import * as Dialog from '@radix-ui/react-dialog'
import * as Progress from '@radix-ui/react-progress'
import * as Switch from '@radix-ui/react-switch'
import * as Tabs from '@radix-ui/react-tabs'
import {
  BookOpen, Check, ChefHat, ChevronRight, CircleGauge, Clock3, CookingPot, Flame,
  Heart, History, ListChecks, Moon, PackageCheck, PoundSterling, RotateCcw,
  Settings2, ShoppingBasket, Sparkles, Star, Sun, UserRound, UsersRound, WifiOff, X,
} from 'lucide-react'
import { api, demoData, type Bootstrap, type Decision, type Rating } from './api'
import { brand } from './brand'

type Screen = 'tonight' | 'recipe' | 'shopping' | 'cooking' | 'profile' | 'history' | 'manage'
const navigation = [
  { id: 'tonight', label: 'Tonight', icon: Sparkles },
  { id: 'recipe', label: 'Recipe', icon: BookOpen },
  { id: 'shopping', label: 'Shopping', icon: ShoppingBasket },
  { id: 'cooking', label: 'Cooking', icon: Flame },
  { id: 'profile', label: 'Profile', icon: UserRound },
  { id: 'history', label: 'History', icon: History },
] satisfies { id: Screen; label: string; icon: typeof Sparkles }[]

function App() {
  const [screen, setScreen] = useState<Screen>('tonight')
  const [data, setData] = useState<Bootstrap>(demoData)
  const [offline, setOffline] = useState(false)
  const [dark, setDark] = useState(() => localStorage.getItem('suppervisor-theme') === 'dark')
  const [brief, setBrief] = useState('Something cosy under 35 minutes, with two lunches')
  const [planning, setPlanning] = useState(false)
  const [notice, setNotice] = useState('')
  const [checked, setChecked] = useState<string[]>([])
  const [step, setStep] = useState(0)
  const [ratingOpen, setRatingOpen] = useState(false)

  useEffect(() => {
    document.documentElement.classList.toggle('dark', dark)
    localStorage.setItem('suppervisor-theme', dark ? 'dark' : 'light')
  }, [dark])
  useEffect(() => { api.bootstrap().then(setData).catch(() => setOffline(true)) }, [])
  const tonight = useMemo(() => data.decisions.find((d) => d.status === 'accepted') ?? data.decisions.find((d) => d.status === 'proposed') ?? data.decisions[0], [data.decisions])

  async function makePlan(event: React.FormEvent) {
    event.preventDefault()
    if (!brief.trim()) return
    setPlanning(true)
    setNotice('')
    try {
      const result = await api.plan(brief)
      setData((current) => ({ ...current, decisions: [...result.decisions, ...current.decisions] }))
      setOffline(false)
    } catch (error) { setNotice(error instanceof Error ? error.message : brand.voice.error) }
    finally { setPlanning(false) }
  }
  async function updateDecision(status: Decision['status'], correction = '') {
    if (!tonight) return
    try {
      if (!offline) await api.setDecision(tonight.id, status, correction)
      setData((current) => ({ ...current, decisions: current.decisions.map((d) => d.id === tonight.id ? { ...d, status, correction } : d) }))
      setNotice(status === 'accepted' ? brand.voice.accepted : 'Noted. Better options next time.')
    } catch (error) { setNotice(error instanceof Error ? error.message : brand.voice.error) }
  }

  return <div className="min-h-dvh bg-canvas text-ink">
    <aside className="sidebar">
      <BrandMark />
      <nav aria-label="Main navigation" className="mt-8 grid gap-1">
        {navigation.map(({ id, label, icon: Icon }) => <button key={id} className="nav-item" aria-current={screen === id ? 'page' : undefined} onClick={() => setScreen(id)}><Icon size={19} /><span>{label}</span></button>)}
      </nav>
      <div className="mt-auto space-y-2 pt-8">
        <button className="nav-item" aria-current={screen === 'manage' ? 'page' : undefined} onClick={() => setScreen('manage')}><Settings2 size={19} /><span>Manage</span></button>
        <div className="theme-row"><Sun size={16} /><span>Kitchen light</span><Switch.Root className="switch" checked={dark} onCheckedChange={setDark} aria-label="Use dark theme"><Switch.Thumb className="switch-thumb" /></Switch.Root><Moon size={16} /></div>
      </div>
    </aside>
    <header className="mobile-header"><BrandMark compact /><button className="icon-button" onClick={() => setDark((value) => !value)} aria-label="Toggle colour theme">{dark ? <Sun size={20} /> : <Moon size={20} />}</button></header>
    <main className="main-shell">
      {offline && <div className="demo-banner" role="status"><WifiOff size={17} />Previewing seeded demo data. Connect the API to make changes stick.</div>}
      {notice && <div className="notice" role="status">{notice}<button onClick={() => setNotice('')} aria-label="Dismiss"><X size={16} /></button></div>}
      {screen === 'tonight' && <Tonight decision={tonight} brief={brief} setBrief={setBrief} planning={planning} onPlan={makePlan} onAccept={() => updateDecision('accepted')} onReject={() => updateDecision('rejected')} onOpen={() => setScreen('recipe')} />}
      {screen === 'recipe' && <Recipe decision={tonight} onShop={() => setScreen('shopping')} onCook={() => setScreen('cooking')} />}
      {screen === 'shopping' && <Shopping decision={tonight} checked={checked} onToggle={(name) => setChecked((items) => items.includes(name) ? items.filter((item) => item !== name) : [...items, name])} />}
      {screen === 'cooking' && <Cooking decision={tonight} step={step} setStep={setStep} onFinish={() => { updateDecision('cooked'); setRatingOpen(true) }} />}
      {screen === 'profile' && <Profiles data={data} />}
      {screen === 'history' && <HistoryView decisions={data.decisions} />}
      {screen === 'manage' && <Management data={data} offline={offline} onRevert={async (id) => { if (!offline) await api.revertPreference(id); setData((current) => ({ ...current, preferenceEvents: current.preferenceEvents.map((event) => event.id === id ? { ...event, revertedAt: new Date().toISOString() } : event) })) }} />}
    </main>
    <nav className="mobile-nav" aria-label="Mobile navigation">{navigation.slice(0, 5).map(({ id, label, icon: Icon }) => <button key={id} aria-current={screen === id ? 'page' : undefined} onClick={() => setScreen(id)}><Icon size={20} /><span>{label}</span></button>)}</nav>
    {tonight && <RatingDialog open={ratingOpen} onOpenChange={setRatingOpen} decision={tonight} profiles={data.profiles} offline={offline} onSaved={(events) => setData((current) => ({ ...current, preferenceEvents: [...events, ...current.preferenceEvents] }))} />}
  </div>
}

function BrandMark({ compact = false }: { compact?: boolean }) {
  return <div className="flex items-center gap-3"><span className="brand-icon"><ChefHat size={compact ? 20 : 24} /></span><span><strong className="font-display text-lg">{brand.name}</strong>{!compact && <small className="block text-xs text-muted">{brand.tagline}</small>}</span></div>
}
function PageIntro({ eyebrow, title, children }: { eyebrow: string; title: string; children?: React.ReactNode }) {
  return <header className="page-intro"><span className="eyebrow">{eyebrow}</span><h1>{title}</h1>{children && <p>{children}</p>}</header>
}
function Tonight({ decision, brief, setBrief, planning, onPlan, onAccept, onReject, onOpen }: { decision?: Decision; brief: string; setBrief: (v: string) => void; planning: boolean; onPlan: (e: React.FormEvent) => void; onAccept: () => void; onReject: () => void; onOpen: () => void }) {
  return <div className="page-grid"><section><PageIntro eyebrow="Tonight" title="What should dinner do for you?">Say it naturally. Suppervisor turns the brief into rules, then ranks what actually fits.</PageIntro><form className="brief-box" onSubmit={onPlan}><label htmlFor="dinner-brief">Dinner brief</label><textarea id="dinner-brief" value={brief} onChange={(e) => setBrief(e.target.value)} rows={3} placeholder="Quick, comforting, no mushrooms, and enough for lunch…" /><div className="flex flex-wrap items-center justify-between gap-3"><div className="flex gap-2 text-xs text-muted"><span className="chip">Alan + Partner</span><span className="chip">Natural language</span></div><button className="primary-button" disabled={planning}>{planning ? 'Sizing up supper…' : 'Find dinner'}<ChevronRight size={18} /></button></div></form></section>{decision ? <CandidateCard decision={decision} onAccept={onAccept} onReject={onReject} onOpen={onOpen} /> : <Empty />}</div>
}
function CandidateCard({ decision, onAccept, onReject, onOpen }: { decision: Decision; onAccept: () => void; onReject: () => void; onOpen: () => void }) {
  const recipe = decision.candidate
  return <article className="hero-card"><div className="hero-card-top"><span className="score-ring">{decision.score.total}<small>fit</small></span><div><span className="eyebrow">{decision.status === 'accepted' ? 'Tonight’s plan' : 'Top match'}</span><h2>{recipe.title}</h2></div></div><p className="mt-4 text-muted">{recipe.summary}</p><div className="metric-row"><span><Clock3 />{recipe.totalMinutes} min</span><span><PoundSterling />{(recipe.costPence / 100).toFixed(2)}</span><span><UsersRound />{recipe.servings} portions</span><span><PackageCheck />Great reheated</span></div>{decision.violations.length > 0 && <div className="warning"><strong>Needs attention</strong>{decision.violations.join(' · ')}</div>}<div className="mt-6 flex flex-wrap gap-2"><button className="primary-button" onClick={onAccept}><Check size={18} />This one</button><button className="secondary-button" onClick={onOpen}>See recipe</button><button className="ghost-button" onClick={onReject}><X size={17} />Not tonight</button></div></article>
}
function Recipe({ decision, onShop, onCook }: { decision?: Decision; onShop: () => void; onCook: () => void }) {
  if (!decision) return <Empty />
  const { candidate } = decision
  return <div><PageIntro eyebrow="Recipe" title={candidate.title}>{candidate.summary}</PageIntro><div className="content-grid"><section className="panel"><h2>What you’ll need</h2><ul className="ingredient-list">{candidate.ingredients.map((item) => <li key={item.name}><span>{item.name}</span><strong>{item.quantity} {item.unit}</strong></li>)}</ul><button className="secondary-button mt-5" onClick={onShop}><ShoppingBasket size={18} />Open shopping</button></section><section className="panel"><h2>The game plan</h2><ol className="step-list">{candidate.steps.map((item, index) => <li key={item.title}><span>{index + 1}</span><div><strong>{item.title}</strong><p>{item.detail}</p></div><small>{item.minutes} min</small></li>)}</ol><button className="primary-button mt-5" onClick={onCook}><Flame size={18} />Start cooking</button></section></div></div>
}
function Shopping({ decision, checked, onToggle }: { decision?: Decision; checked: string[]; onToggle: (name: string) => void }) {
  if (!decision) return <Empty />
  const items = decision.candidate.ingredients
  const complete = items.length ? checked.length / items.length * 100 : 0
  return <div><PageIntro eyebrow="Shopping" title="A short, useful list">{checked.length} of {items.length} tucked into the basket.</PageIntro><Progress.Root className="progress-track" value={complete}><Progress.Indicator className="progress-fill" style={{ transform: `translateX(-${100 - complete}%)` }} /></Progress.Root><div className="panel mt-5"><ul className="shopping-list">{items.map((item) => <li key={item.name}><label><input type="checkbox" checked={checked.includes(item.name)} onChange={() => onToggle(item.name)} /><span className="checkmark"><Check size={15} /></span><span className={checked.includes(item.name) ? 'line-through text-muted' : ''}>{item.name}</span><strong>{item.quantity} {item.unit}</strong></label></li>)}</ul></div></div>
}
function Cooking({ decision, step, setStep, onFinish }: { decision?: Decision; step: number; setStep: (v: number) => void; onFinish: () => void }) {
  if (!decision) return <Empty />
  const steps = decision.candidate.steps
  const current = steps[Math.min(step, steps.length - 1)]
  return <div className="cooking-shell"><PageIntro eyebrow="Cooking" title={brand.voice.cooking}>{decision.candidate.title}</PageIntro><div className="cook-progress">{steps.map((item, index) => <button key={item.title} className={index <= step ? 'active' : ''} onClick={() => setStep(index)} aria-label={`Go to step ${index + 1}`}>{index < step ? <Check size={15} /> : index + 1}</button>)}</div><article className="cook-card"><span className="eyebrow">Step {step + 1} of {steps.length} · {current.minutes} minutes</span><h2>{current.title}</h2><p>{current.detail}</p><div className="mt-8 flex justify-between gap-3"><button className="secondary-button" disabled={step === 0} onClick={() => setStep(Math.max(0, step - 1))}>Back</button>{step < steps.length - 1 ? <button className="primary-button" onClick={() => setStep(step + 1)}>Next step<ChevronRight size={18} /></button> : <button className="primary-button" onClick={onFinish}><Check size={18} />Dinner done</button>}</div></article></div>
}
function Profiles({ data }: { data: Bootstrap }) {
  return <div><PageIntro eyebrow="Profiles" title="Different people, separate tastes">Hard dislikes stay hard. Softer preferences can learn from every rating.</PageIntro><div className="card-grid">{data.profiles.map((profile) => <article className="panel" key={profile.id}><div className="profile-heading"><span>{profile.name[0]}</span><div><h2>{profile.name}</h2><small>Individual dinner profile</small></div></div><h3>Known winners</h3><div className="tag-row">{profile.likes.map((item) => <span className="positive-tag" key={item}><Heart size={13} />{item}</span>)}</div><h3>Hard no, thanks</h3><div className="tag-row">{profile.dislikes.map((item) => <span className="negative-tag" key={item}>{item}</span>)}</div><h3>Learned leanings</h3>{Object.entries(profile.preferences).map(([key, value]) => <div className="preference-bar" key={key}><span>{key}</span><div><i style={{ width: `${Math.min(100, 45 + value * 25)}%` }} /></div><strong>{value > 0 ? '+' : ''}{value.toFixed(2)}</strong></div>)}</article>)}</div></div>
}
function HistoryView({ decisions }: { decisions: Decision[] }) {
  return <div><PageIntro eyebrow="History" title="Dinner’s paper trail">What won, what didn’t, and why. No mysterious recommendations.</PageIntro><div className="timeline">{decisions.map((d) => <article key={d.id}><span className={`status-dot ${d.status}`} /><div><small>{new Date(d.createdAt).toLocaleDateString()} · {d.status}</small><h2>{d.candidate.title}</h2><p>{d.request.text}</p>{d.correction && <p className="correction">Correction: {d.correction}</p>}</div><strong>{d.score.total}</strong></article>)}</div></div>
}
function Management({ data, offline, onRevert }: { data: Bootstrap; offline: boolean; onRevert: (id: string) => void }) {
  const tabs = ['profiles', 'preferences', 'meals', 'ratings', 'recipes', 'decisions', 'status', 'corrections']
  return <div><PageIntro eyebrow="Management" title="Everything with the lid lifted">Profiles, recipes, decisions and learning — inspectable and correctable.</PageIntro><Tabs.Root defaultValue="profiles" className="manage-tabs"><Tabs.List aria-label="Management sections">{tabs.map((tab) => <Tabs.Trigger key={tab} value={tab}>{tab}</Tabs.Trigger>)}</Tabs.List><Tabs.Content value="profiles"><DataTable rows={data.profiles.map((p) => [p.name, p.likes.join(', '), p.dislikes.join(', ')])} headings={['Profile', 'Likes', 'Dislikes']} /></Tabs.Content><Tabs.Content value="preferences"><div className="stack-list">{data.preferenceEvents.length ? data.preferenceEvents.map((event) => <article key={event.id}><div><strong>{event.profileId} · {event.key}</strong><p>{event.reason}</p></div><span className={event.delta >= 0 ? 'good' : 'bad'}>{event.delta > 0 ? '+' : ''}{event.delta}</span><button className="icon-button" disabled={!!event.revertedAt} onClick={() => onRevert(event.id)} aria-label={`Revert ${event.key} preference`}><RotateCcw size={17} /></button></article>) : <Empty compact />}</div></Tabs.Content><Tabs.Content value="meals"><DataTable rows={data.decisions.filter((d) => ['accepted', 'cooked'].includes(d.status)).map((d) => [d.candidate.title, d.status, new Date(d.createdAt).toLocaleDateString()])} headings={['Meal', 'State', 'Date']} /></Tabs.Content><Tabs.Content value="ratings">{data.ratings?.length ? <DataTable rows={data.ratings.map((r) => [r.profileId, `${r.overall}/5`, r.notes])} headings={['Profile', 'Overall', 'Notes']} /> : <Empty compact />}</Tabs.Content><Tabs.Content value="recipes"><DataTable rows={data.recipes.map((r) => [r.title, `${r.totalMinutes} min`, `£${(r.costPence / 100).toFixed(2)}`])} headings={['Recipe', 'Time', 'Cost']} /></Tabs.Content><Tabs.Content value="decisions"><DataTable rows={data.decisions.map((d) => [d.candidate.title, d.status, `${d.score.total} fit`])} headings={['Decision', 'State', 'Score']} /></Tabs.Content><Tabs.Content value="status"><div className="status-grid"><StatusCard icon={CircleGauge} title="API" value={offline ? 'Demo mode' : 'Connected'} /><StatusCard icon={CookingPot} title="Generation" value={offline ? 'Seeded fallback' : 'Ollama via server'} /><StatusCard icon={ListChecks} title="Rules" value="Deterministic" /></div></Tabs.Content><Tabs.Content value="corrections"><div className="stack-list">{data.decisions.filter((d) => d.correction).map((d) => <article key={d.id}><div><strong>{d.candidate.title}</strong><p>{d.correction}</p></div><span className="chip">adjusted</span></article>)}{!data.decisions.some((d) => d.correction) && <Empty compact />}</div></Tabs.Content></Tabs.Root></div>
}
function DataTable({ headings, rows }: { headings: string[]; rows: string[][] }) {
  return <div className="table-wrap"><table><thead><tr>{headings.map((h) => <th key={h}>{h}</th>)}</tr></thead><tbody>{rows.map((row, i) => <tr key={i}>{row.map((cell, j) => <td key={j}>{cell}</td>)}</tr>)}</tbody></table>{rows.length === 0 && <Empty compact />}</div>
}
function StatusCard({ icon: Icon, title, value }: { icon: typeof CircleGauge; title: string; value: string }) {
  return <article className="panel"><Icon size={22} /><small>{title}</small><strong>{value}</strong></article>
}
function RatingDialog({ open, onOpenChange, decision, profiles, offline, onSaved }: { open: boolean; onOpenChange: (v: boolean) => void; decision: Decision; profiles: Bootstrap['profiles']; offline: boolean; onSaved: (events: Bootstrap['preferenceEvents']) => void }) {
  const [profileId, setProfileId] = useState(profiles[0]?.id ?? 'alan')
  const [overall, setOverall] = useState(4)
  const [notes, setNotes] = useState('')
  async function save() { const rating: Rating = { decisionId: decision.id, profileId, overall, taste: overall, ease: overall, reheat: overall, tags: [], notes }; if (!offline) { const result = await api.rate(rating); onSaved(result.preferenceEvents) }; onOpenChange(false) }
  return <Dialog.Root open={open} onOpenChange={onOpenChange}><Dialog.Portal><Dialog.Overlay className="dialog-overlay" /><Dialog.Content className="dialog-content"><Dialog.Title>How did dinner land?</Dialog.Title><Dialog.Description>Rate your own plate. Alan and Partner never share a score.</Dialog.Description><label>Who’s rating?<select value={profileId} onChange={(e) => setProfileId(e.target.value)}>{profiles.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}</select></label><fieldset><legend>Overall</legend><div className="star-row">{[1,2,3,4,5].map((v) => <button key={v} onClick={() => setOverall(v)} aria-label={`${v} stars`}><Star fill={v <= overall ? 'currentColor' : 'none'} /></button>)}</div></fieldset><label>Anything worth remembering?<textarea value={notes} onChange={(e) => setNotes(e.target.value)} placeholder="The crispy bits disappeared first…" /></label><div className="flex justify-end gap-2"><Dialog.Close className="secondary-button">Later</Dialog.Close><button className="primary-button" onClick={save}>Save rating</button></div><Dialog.Close className="dialog-x" aria-label="Close"><X /></Dialog.Close></Dialog.Content></Dialog.Portal></Dialog.Root>
}
function Empty({ compact = false }: { compact?: boolean }) {
  return <div className={compact ? 'empty-compact' : 'empty-card'}><CookingPot size={compact ? 22 : 32} /><p>{brand.voice.emptyTonight}</p></div>
}

export default App
