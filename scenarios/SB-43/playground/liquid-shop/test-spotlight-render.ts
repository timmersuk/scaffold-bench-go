import { Liquid } from "liquidjs";
import { readFileSync } from "fs";
import { join } from "path";

const dir = import.meta.dir;
const content = readFileSync(join(dir, "sections/product-spotlight.liquid"), "utf-8");
const templateOnly = content.replace(/\{%-?\s*schema[\s\S]*?endschema\s*-?%\}/g, "");

const engine = new Liquid();
engine.registerFilter("money", (v: number) => `$${(v / 100).toFixed(2)}`);

const products = [
  { id: 1, title: "Widget A", available: true, price: 1999 },
  { id: 2, title: "Widget B", available: true, price: 2999 },
  { id: 3, title: "Widget C", available: true, price: 999 },
  { id: 4, title: "Widget D", available: true, price: 3499 },
  { id: 5, title: "Widget E", available: true, price: 599 },
];

const withPrice = await engine.parseAndRender(templateOnly, {
  products,
  section: { settings: { heading: "Spotlight", product_limit: 4, show_price: true } },
});

const cardCount = (withPrice.match(/spotlight-card|product-card/g) ?? []).length;
if (cardCount < 1) {
  console.error("FAIL: no cards rendered");
  process.exit(1);
}

if (!/\$\d+\.\d{2}/.test(withPrice)) {
  console.error("FAIL: no prices in output when show_price=true");
  process.exit(1);
}

const noPrice = await engine.parseAndRender(templateOnly, {
  products,
  section: { settings: { heading: "Spotlight", product_limit: 4, show_price: false } },
});

if (/\$\d+\.\d{2}/.test(noPrice)) {
  console.error("FAIL: prices shown when show_price=false");
  process.exit(1);
}

console.log("PASS: rendering correct, price toggle works");
