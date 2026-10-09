# Suppervisor product goal

## Promise

**Suppervisor — Dinner, handled.**

Turn an ordinary, messy dinner request into a meal that works for the actual diners, then help them shop, prep, cook, and learn from the result without sending household preference data to a hosted AI.

## Complete product loop

1. Alan or Partner asks naturally from Telegram or the PWA.
2. Suppervisor exposes the structured interpretation: diners, portions, lunches, reheating, ingredients, maximum time and cost, equipment, and exclusions.
3. Local Ollama proposes concrete candidates through a provider-neutral boundary and gets one repair attempt for invalid output.
4. Deterministic code rejects hard-rule failures and scores valid options against individual preferences and recent meals.
5. A person accepts, rejects, or adjusts the decision; the reason remains inspectable.
6. Suppervisor creates the shopping list and guides prep and cooking one step at a time.
7. Each diner rates taste, ease, reheating, tags, and notes separately.
8. Ratings create auditable preference events that can be reverted.

## Guardrails

- No hidden relaxation of dislikes or safety constraints.
- No shared “household average” that erases individual preferences.
- No cloud model requirement.
- No irreversible learned state.
- No production auth bypass.
- No iteration-one worker process; one modular-monolith process owns HTTP and Telegram delivery.
- PostgreSQL is the durable source of truth. Production PostgreSQL is external and backed up independently.

## Iteration-one success

A local Compose user can submit a dinner brief, receive ranked candidates, make a decision, shop, cook, and submit two individual ratings. The preference changes and every decision can be inspected in management views. The same images can be deployed with the Helm chart to Talos Kubernetes using external PostgreSQL, Keycloak OIDC, secret references, and optional in-cluster Ollama.
