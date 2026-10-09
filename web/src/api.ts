export type Profile = {
  id: string;
  name: string;
  dislikes: string[];
  likes: string[];
  preferences: Record<string, number>;
};

export type Ingredient = {
  name: string;
  quantity: number;
  unit: string;
  optional?: boolean;
};
export type RecipeStep = { title: string; detail: string; minutes: number };
export type Candidate = {
  id: string;
  source?: string;
  title: string;
  summary: string;
  servings: number;
  activeMinutes: number;
  totalMinutes: number;
  costPence: number;
  ingredients: Ingredient[];
  equipment: string[];
  tags: string[];
  reheatsWell: boolean;
  steps: RecipeStep[];
};

export type Decision = {
  id: string;
  request: {
    schemaVersion: number;
    text: string;
    diners: string[];
    servings: number;
    lunchPortions: number;
    maxMinutes: number;
    maxCostPence: number;
    available?: string[];
    assumptions?: string[];
  };
  candidate: Candidate;
  score: {
    preference: number;
    fairness?: number;
    byDiner?: Record<string, number>;
    time: number;
    cost: number;
    leftovers: number;
    novelty: number;
    ingredients: number;
    total: number;
  };
  violations: string[];
  status: "proposed" | "accepted" | "rejected" | "adjusted" | "cooked";
  correction?: string;
  createdAt: string;
};

export type PreferenceEvent = {
  id: string;
  profileId: string;
  key: string;
  delta: number;
  reason: string;
  ratingId?: string;
  reversalOf?: string;
  revertedAt?: string;
  createdAt: string;
};

export type DecisionEvent = {
  id: string;
  decisionId: string;
  type: Decision["status"];
  actor: string;
  detail: Record<string, unknown>;
  createdAt: string;
};

export type Rating = {
  decisionId: string;
  profileId: string;
  overall: number;
  taste: number;
  ease: number;
  reheat: number;
  tags: string[];
  notes: string;
};

export type WorkflowState = {
  decisionId: string;
  shoppingChecked: string[];
  cookingStep: number;
  updatedAt: string;
};

export type Bootstrap = {
  profiles: Profile[];
  recipes: Candidate[];
  decisions: Decision[];
  decisionEvents: DecisionEvent[];
  workflowStates: WorkflowState[];
  preferenceEvents: PreferenceEvent[];
  ratings: Rating[];
};

const API = import.meta.env.VITE_API_URL ?? "/api/v1";

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const token = sessionStorage.getItem("suppervisor-token");
  const response = await fetch(`${API}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...init?.headers,
    },
  });
  if (!response.ok) {
    const body = await response
      .json()
      .catch(() => ({ error: response.statusText }));
    throw new Error(body.error ?? "Request failed");
  }
  if (response.status === 204) return undefined as T;
  return response.json();
}

export const api = {
  bootstrap: () => request<Bootstrap>("/bootstrap"),
  plan: (text: string) =>
    request<{ decisions: Decision[] }>("/plans", {
      method: "POST",
      body: JSON.stringify({ text }),
    }),
  setDecision: (id: string, status: Decision["status"], correction = "") =>
    request<{ decisions?: Decision[] } | undefined>(`/decisions/${id}`, {
      method: "PATCH",
      body: JSON.stringify({ status, correction }),
    }),
  setWorkflow: (id: string, shoppingChecked: string[], cookingStep: number) =>
    request<WorkflowState>(`/decisions/${id}/workflow`, {
      method: "PATCH",
      body: JSON.stringify({ shoppingChecked, cookingStep }),
    }),
  rate: (rating: Rating) =>
    request<{ preferenceEvents: PreferenceEvent[] }>("/ratings", {
      method: "POST",
      body: JSON.stringify(rating),
    }),
  revertPreference: (id: string) =>
    request<void>(`/preferences/events/${id}/revert`, { method: "POST" }),
  status: () => request<Record<string, unknown>>("/status"),
};

export const demoData: Bootstrap = {
  profiles: [
    {
      id: "alan",
      name: "Alan",
      dislikes: ["aubergine"],
      likes: ["chicken thighs", "beans", "traybakes"],
      preferences: { mediterranean: 1.25, mexican: 0.75, "one-pan": 1 },
    },
    {
      id: "partner",
      name: "Partner",
      dislikes: ["mushrooms", "blue cheese"],
      likes: ["orzo", "roast peppers", "ginger"],
      preferences: { italian: 1.25, asian: 0.75, vegetarian: 0.5 },
    },
  ],
  recipes: [],
  decisionEvents: [
    {
      id: "demo-decision-event",
      decisionId: "demo-tonight",
      type: "accepted",
      actor: "demo",
      detail: { note: "development-only accepted meal" },
      createdAt: new Date().toISOString(),
    },
  ],
  workflowStates: [
    {
      decisionId: "demo-tonight",
      shoppingChecked: [],
      cookingStep: 0,
      updatedAt: new Date().toISOString(),
    },
  ],
  preferenceEvents: [],
  ratings: [],
  decisions: [
    {
      id: "demo-tonight",
      request: {
        schemaVersion: 1,
        text: "Something cosy under 35 minutes, with two lunches",
        diners: ["alan", "partner"],
        servings: 2,
        lunchPortions: 2,
        maxMinutes: 35,
        maxCostPence: 1200,
        available: [],
        assumptions: ["standard hob and oven available"],
      },
      candidate: {
        id: "tomato-lentil-orzo",
        title: "One-pan tomato lentil orzo",
        summary: "Silky tomato orzo, earthy lentils and a bright basil finish.",
        servings: 4,
        activeMinutes: 10,
        totalMinutes: 28,
        costPence: 690,
        reheatsWell: true,
        equipment: ["hob"],
        tags: ["italian", "vegetarian", "one-pan"],
        ingredients: [
          { name: "orzo", quantity: 300, unit: "g" },
          { name: "green lentils", quantity: 1, unit: "tin" },
          { name: "chopped tomatoes", quantity: 1, unit: "tin" },
          { name: "spinach", quantity: 150, unit: "g" },
          { name: "basil", quantity: 1, unit: "handful" },
        ],
        steps: [
          {
            title: "Build the base",
            detail: "Soften garlic, then add tomatoes, lentils and orzo.",
            minutes: 8,
          },
          {
            title: "Simmer",
            detail: "Add stock and simmer, stirring, until silky.",
            minutes: 15,
          },
          {
            title: "Green finish",
            detail: "Fold through spinach and basil; season generously.",
            minutes: 5,
          },
        ],
      },
      score: {
        preference: 32,
        time: 17,
        cost: 13,
        leftovers: 10,
        novelty: 9,
        ingredients: 11,
        total: 92,
      },
      violations: [],
      status: "accepted",
      createdAt: new Date().toISOString(),
    },
  ],
};
demoData.recipes = demoData.decisions.map((decision) => decision.candidate);
