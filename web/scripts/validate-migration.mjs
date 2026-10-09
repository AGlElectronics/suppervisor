import { readFile } from "node:fs/promises";
import { PGlite } from "@electric-sql/pglite";

const migrationUrl = new URL(
  "../../internal/store/migrations/00001_initial.sql",
  import.meta.url,
);
const sql = await readFile(migrationUrl, "utf8");
const upMark = "-- +suppervisor Up";
const downMark = "-- +suppervisor Down";
const upAt = sql.indexOf(upMark);
const downAt = sql.indexOf(downMark);
if (upAt < 0 || downAt <= upAt)
  throw new Error("migration markers are invalid");
const up = sql.slice(upAt + upMark.length, downAt).trim();
const down = sql.slice(downAt + downMark.length).trim();

const db = new PGlite();
await db.exec(up);

const requiredTables = [
  "decision_events",
  "decisions",
  "preference_events",
  "profiles",
  "ratings",
  "recipes",
  "schema_migrations",
  "telegram_updates",
];
const tables = await db.query(
  "SELECT table_name FROM information_schema.tables WHERE table_schema='public'",
);
const actual = new Set(tables.rows.map((row) => row.table_name));
for (const table of requiredTables) {
  if (!actual.has(table)) throw new Error(`missing table after up: ${table}`);
}

await db.exec(`
  INSERT INTO decisions(id,request,recipe_id,score,status)
  VALUES(
    '10000000-0000-4000-8000-000000000001',
    '{"schemaVersion":1,"text":"migration smoke"}',
    'known-winner-tomato-orzo',
    '{"total":90}',
    'accepted'
  );
  INSERT INTO ratings(id,decision_id,profile_id,overall,taste,ease,reheat)
  VALUES(
    '10000000-0000-4000-8000-000000000002',
    '10000000-0000-4000-8000-000000000001',
    'alan',5,5,4,4
  );
  INSERT INTO preference_events(id,profile_id,key,delta,reason,rating_id)
  VALUES(
    '10000000-0000-4000-8000-000000000003',
    'alan','italian',0.5,'migration smoke',
    '10000000-0000-4000-8000-000000000002'
  );
  INSERT INTO preference_events(id,profile_id,key,delta,reason,rating_id,reversal_of)
  VALUES(
    '10000000-0000-4000-8000-000000000004',
    'alan','italian',-0.5,'migration smoke reversal',
    '10000000-0000-4000-8000-000000000002',
    '10000000-0000-4000-8000-000000000003'
  );
`);
const balance = await db.query(
  "SELECT SUM(delta)::float8 AS balance FROM preference_events WHERE key='italian'",
);
if (balance.rows[0].balance !== 0) {
  throw new Error(
    `preference compensation did not balance: ${balance.rows[0].balance}`,
  );
}

await db.exec(down);
const remaining = await db.query(
  "SELECT table_name FROM information_schema.tables WHERE table_schema='public'",
);
if (remaining.rows.length !== 0) {
  throw new Error(
    `down migration left tables: ${remaining.rows.map((row) => row.table_name).join(", ")}`,
  );
}

await db.close();
console.log(
  "PGlite migration validation passed: up, constraints, compensating event, and down",
);
