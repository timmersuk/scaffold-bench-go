import { Liquid } from "liquidjs";
import { readFileSync } from "fs";
import { join } from "path";

const dir = import.meta.dir;
const content = readFileSync(join(dir, "sections/featured-grid.liquid"), "utf-8");
const templateOnly = content.replace(/\{%[-\s]*schema[\s\S]*?endschema[\s\S]*?%\}/g, "");

const engine = new Liquid();
engine.registerFilter("money", (v: number) => `$${(v / 100).toFixed(2)}`);

const products = [
  { id: 1, title: "Widget A", available: true, price: 1999 },
  { id: 2, title: "Widget B", available: false, price: 2999 },
];

const result = await engine.parseAndRender(templateOnly, {
  products,
  section: { settings: { show_soldout: true } },
});

const count = (result.match(/product-card/g) ?? []).length;
if (count !== 2) {
  console.error(`FAIL: expected 2 cards, got ${count}`);
  process.exit(1);
}
console.log("PASS: all products shown when show_soldout=true");
