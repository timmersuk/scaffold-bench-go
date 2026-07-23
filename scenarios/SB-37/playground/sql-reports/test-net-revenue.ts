import { Database } from "bun:sqlite";
import { readFileSync } from "fs";
import { join } from "path";

const dir = import.meta.dir;
const db = new Database(":memory:");
db.exec(readFileSync(join(dir, "schema.sql"), "utf-8"));
db.exec(readFileSync(join(dir, "seed.sql"), "utf-8"));
const sql = readFileSync(join(dir, "queries/monthly-net-revenue.sql"), "utf-8");
if (!sql.trim()) { console.error("FAIL: query file is empty"); process.exit(1); }
const rows = db.query(sql).all() as { client_id: number; month: string; net_revenue: number }[];
const client1Jan = rows.find(r => r.client_id === 1 && r.month === "2024-01");
if (!client1Jan || Math.abs(client1Jan.net_revenue - 450) > 0.01) {
  console.error(`FAIL: client 1 jan net_revenue = ${client1Jan?.net_revenue}, expected 450`);
  process.exit(1);
}
console.log("PASS: dataset 1 correct");
