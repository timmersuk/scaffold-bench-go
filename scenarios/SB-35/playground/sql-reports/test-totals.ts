import { Database } from "bun:sqlite";
import { readFileSync } from "fs";
import { join } from "path";

const dir = import.meta.dir;
const db = new Database(":memory:");
db.exec(readFileSync(join(dir, "schema.sql"), "utf-8"));
db.exec(readFileSync(join(dir, "seed.sql"), "utf-8"));
const sql = readFileSync(join(dir, "queries/totals.sql"), "utf-8");
const rows = db.query(sql).all() as { client_id: number; total: number }[];
const client1 = rows.find(r => r.client_id === 1);
if (!client1 || Math.abs(client1.total - 350) > 0.01) {
  console.error(`FAIL: client 1 total = ${client1?.total}, expected 350`);
  process.exit(1);
}
console.log("PASS");
