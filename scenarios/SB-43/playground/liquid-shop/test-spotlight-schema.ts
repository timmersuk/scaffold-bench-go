import { readFileSync } from "fs";
import { join } from "path";

const dir = import.meta.dir;
const content = readFileSync(join(dir, "sections/product-spotlight.liquid"), "utf-8");

const match = content.match(/\{%-?\s*schema\s*-?%\}([\s\S]*?)\{%-?\s*endschema\s*-?%\}/);
if (!match) {
  console.error("FAIL: no valid {% schema %} block found");
  process.exit(1);
}

let schema: any;
try {
  schema = JSON.parse(match[1].trim());
} catch {
  console.error("FAIL: schema block is not valid JSON");
  process.exit(1);
}

const settings = schema.settings ?? [];
const heading = settings.find((s: any) => s.type === "text" && s.id === "heading");
const limit = settings.find((s: any) => (s.type === "range" || s.type === "number") && s.id === "product_limit");
const price = settings.find((s: any) => s.type === "checkbox" && s.id === "show_price");

if (!heading || !limit || !price) {
  console.error(`FAIL: missing settings: heading=${!!heading} limit=${!!limit} price=${!!price}`);
  process.exit(1);
}

if (limit.default !== 4) {
  console.error(`FAIL: product_limit default is ${limit.default}, expected 4`);
  process.exit(1);
}

console.log("PASS: schema valid with all required settings and defaults");
