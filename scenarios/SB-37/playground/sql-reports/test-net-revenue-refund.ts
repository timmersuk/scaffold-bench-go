import { Database } from "bun:sqlite";
import { readFileSync } from "fs";
import { join } from "path";

const dir = import.meta.dir;
const db = new Database(":memory:");
db.exec(readFileSync(join(dir, "schema.sql"), "utf-8"));
db.exec(readFileSync(join(dir, "seed.sql"), "utf-8"));
db.exec("INSERT INTO refunds (id, client_id, amount, month) VALUES (99, 2, 200.00, '2024-02');");
const sql = readFileSync(join(dir, "queries/monthly-net-revenue.sql"), "utf-8");
if (!sql.trim()) { console.error("FAIL: query file is empty"); process.exit(1); }
const rows = db.query(sql).all() as { client_id: number; month: string; net_revenue: number }[];
const client2Feb = rows.find(r => r.client_id === 2 && r.month === "2024-02");
if (!client2Feb || Math.abs(client2Feb.net_revenue - (-200)) > 0.01) {
  console.error(`FAIL: client 2 feb net_revenue = ${client2Feb?.net_revenue}, expected -200`);
  process.exit(1);
}
console.log("PASS: dataset 2 correct (refund-only month)");
