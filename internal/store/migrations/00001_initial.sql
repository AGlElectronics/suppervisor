-- +suppervisor Up
CREATE TABLE IF NOT EXISTS schema_migrations (
  version text PRIMARY KEY,
  applied_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE profiles (
  id text PRIMARY KEY,
  name text NOT NULL,
  dislikes jsonb NOT NULL DEFAULT '[]',
  likes jsonb NOT NULL DEFAULT '[]',
  base_preferences jsonb NOT NULL DEFAULT '{}',
  preferences jsonb NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE recipes (
  id text PRIMARY KEY,
  title text NOT NULL,
  body jsonb NOT NULL,
  source text NOT NULL DEFAULT 'generated',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE decisions (
  id uuid PRIMARY KEY,
  request jsonb NOT NULL,
  recipe_id text NOT NULL REFERENCES recipes(id),
  score jsonb NOT NULL,
  violations jsonb NOT NULL DEFAULT '[]',
  status text NOT NULL CHECK (status IN ('proposed','accepted','rejected','adjusted','cooked')),
  correction text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX decisions_created_idx ON decisions (created_at DESC);

CREATE TABLE workflow_states (
  decision_id uuid PRIMARY KEY REFERENCES decisions(id) ON DELETE CASCADE,
  shopping_checked jsonb NOT NULL DEFAULT '[]'
    CHECK (jsonb_typeof(shopping_checked) = 'array'),
  cooking_step integer NOT NULL DEFAULT 0 CHECK (cooking_step >= 0),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE decision_events (
  id uuid PRIMARY KEY,
  decision_id uuid NOT NULL REFERENCES decisions(id),
  event_type text NOT NULL CHECK (event_type IN ('proposed','accepted','rejected','adjusted','cooked')),
  actor text NOT NULL DEFAULT 'user',
  detail jsonb NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX decision_events_decision_idx ON decision_events (decision_id, created_at);

CREATE TABLE ratings (
  id uuid PRIMARY KEY,
  decision_id uuid NOT NULL REFERENCES decisions(id),
  profile_id text NOT NULL REFERENCES profiles(id),
  overall smallint NOT NULL CHECK (overall BETWEEN 1 AND 5),
  taste smallint NOT NULL CHECK (taste BETWEEN 1 AND 5),
  ease smallint NOT NULL CHECK (ease BETWEEN 1 AND 5),
  reheat smallint NOT NULL CHECK (reheat BETWEEN 1 AND 5),
  tags jsonb NOT NULL DEFAULT '[]',
  notes text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (decision_id, profile_id)
);

CREATE TABLE preference_events (
  id uuid PRIMARY KEY,
  profile_id text NOT NULL REFERENCES profiles(id),
  key text NOT NULL,
  delta numeric NOT NULL,
  reason text NOT NULL,
  rating_id uuid REFERENCES ratings(id),
  reversal_of uuid UNIQUE REFERENCES preference_events(id),
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX preference_events_profile_idx ON preference_events (profile_id, created_at DESC);

CREATE TABLE telegram_updates (
  update_id bigint PRIMARY KEY,
  received_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO profiles (id, name, dislikes, likes, base_preferences, preferences) VALUES
  ('alan', 'Alan', '["aubergine"]', '["chicken thighs","beans","traybakes"]', '{"mediterranean":1.25,"mexican":0.75,"one-pan":1}', '{"mediterranean":1.25,"mexican":0.75,"one-pan":1}'),
  ('partner', 'Partner', '["mushrooms","blue cheese"]', '["orzo","roast peppers","ginger"]', '{"italian":1.25,"asian":0.75,"vegetarian":0.5}', '{"italian":1.25,"asian":0.75,"vegetarian":0.5}')
ON CONFLICT (id) DO NOTHING;

INSERT INTO recipes (id, title, source, body) VALUES
  ('known-winner-tomato-orzo', 'One-pan tomato lentil orzo', 'known-winner', '{"id":"known-winner-tomato-orzo","title":"One-pan tomato lentil orzo","summary":"A known winner: silky tomato orzo with lentils and greens.","servings":4,"activeMinutes":10,"totalMinutes":28,"costPence":690,"ingredients":[{"name":"orzo","quantity":300,"unit":"g"},{"name":"green lentils","quantity":1,"unit":"tin"},{"name":"chopped tomatoes","quantity":1,"unit":"tin"},{"name":"spinach","quantity":150,"unit":"g"}],"equipment":["hob"],"tags":["italian","vegetarian","one-pan"],"reheatsWell":true,"steps":[{"title":"Build the base","detail":"Add tomatoes, lentils and orzo to softened garlic.","minutes":8},{"title":"Simmer","detail":"Add stock and simmer until silky.","minutes":15},{"title":"Finish","detail":"Fold through spinach and season.","minutes":5}]}'),
  ('known-winner-chicken-bowls', 'Sticky ginger chicken rice bowls', 'known-winner', '{"id":"known-winner-chicken-bowls","title":"Sticky ginger chicken rice bowls","summary":"A known winner: savoury ginger chicken and crisp cucumber.","servings":4,"activeMinutes":18,"totalMinutes":35,"costPence":1150,"ingredients":[{"name":"chicken thighs","quantity":600,"unit":"g"},{"name":"rice","quantity":300,"unit":"g"},{"name":"cucumber","quantity":1,"unit":""},{"name":"fresh ginger","quantity":25,"unit":"g"}],"equipment":["hob"],"tags":["asian","chicken","rice"],"reheatsWell":true,"steps":[{"title":"Cook rice","detail":"Rinse and cook rice.","minutes":18},{"title":"Glaze chicken","detail":"Brown chicken with ginger and soy.","minutes":12},{"title":"Build bowls","detail":"Slice cucumber and assemble.","minutes":5}]}')
ON CONFLICT (id) DO NOTHING;

-- +suppervisor Down
DROP TABLE IF EXISTS telegram_updates;
DROP TABLE IF EXISTS preference_events;
DROP TABLE IF EXISTS ratings;
DROP TABLE IF EXISTS decision_events;
DROP TABLE IF EXISTS workflow_states;
DROP TABLE IF EXISTS decisions;
DROP TABLE IF EXISTS recipes;
DROP TABLE IF EXISTS profiles;
DROP TABLE IF EXISTS schema_migrations;
