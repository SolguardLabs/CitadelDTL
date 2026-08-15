import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));
const src = join(root, "src");
const min = 3000;
let lines = 0;

function walk(dir) {
  for (const entry of readdirSync(dir)) {
    const path = join(dir, entry);
    const stat = statSync(path);
    if (stat.isDirectory()) {
      walk(path);
      continue;
    }
    if (!path.endsWith(".go")) {
      continue;
    }
    const text = readFileSync(path, "utf8");
    lines += text
      .split(/\r?\n/)
      .filter((line) => line.trim().length > 0).length;
  }
}

walk(src);
console.log(`src LOC: ${lines}`);
if (lines < min) {
  console.error(`expected at least ${min} non-empty src lines`);
  process.exit(1);
}
