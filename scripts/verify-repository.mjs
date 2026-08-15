import { createHash } from "node:crypto";
import { existsSync, readdirSync, readFileSync, statSync } from "node:fs";
import { extname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));
const expectedBannerHash =
  "c9480cef4d7a69c4169e18fce27d9a7f7f305988c6e0255bde1c8e56db602cc6";
const required = [
  "README.md",
  "SECURITY.md",
  "assets/banner.png",
  "sdk/citadelClient.ts",
  "src/governance/executor.go",
  "src/risk/capital.go",
];

for (const entry of required) {
  if (!existsSync(join(root, entry)))
    fail(`missing required artifact: ${entry}`);
}

const docs = readdirSync(join(root, "docs")).filter((name) =>
  name.endsWith(".md"),
);
if (docs.length !== 7)
  fail(`expected exactly 7 operational documents, found ${docs.length}`);

const bannerHash = createHash("sha256")
  .update(readFileSync(join(root, "assets", "banner.png")))
  .digest("hex");
if (bannerHash !== expectedBannerHash)
  fail(`unexpected banner digest: ${bannerHash}`);

const readableExtensions = new Set([
  ".go",
  ".md",
  ".mjs",
  ".ts",
  ".json",
  ".yml",
  ".yaml",
]);
const excluded = new Set([".git", "node_modules", "tests/private"]);
const forbiddenTerms = [
  "c" + "tf",
  "la" + "boratorio",
  "l" + "a" + "b",
  "vulnera" + "bilidad",
  "vulnera" + "ble",
  "vulnera" + "bility",
  "b" + "ug",
  "ex" + "ploit",
  "by" + "pass",
  "at" + "tacker",
];
let mermaidBlocks = 0;

for (const path of walk(root)) {
  const name = relative(root, path).replaceAll("\\", "/");
  if (name === "package-lock.json" || !readableExtensions.has(extname(path)))
    continue;
  const text = readFileSync(path, "utf8");
  mermaidBlocks += (text.match(/```mermaid\b/g) ?? []).length;
  for (const term of forbiddenTerms) {
    const expression = new RegExp(`\\b${escapeRegExp(term)}s?\\b`, "iu");
    if (expression.test(text)) fail(`restricted public terminology in ${name}`);
  }
}
if (mermaidBlocks < 18)
  fail(`expected at least 18 Mermaid diagrams, found ${mermaidBlocks}`);

console.log(
  `repository contract passed: ${docs.length} docs, ${mermaidBlocks} Mermaid diagrams, banner ${bannerHash.slice(0, 12)}`,
);

function* walk(directory) {
  for (const entry of readdirSync(directory)) {
    const path = join(directory, entry);
    const name = relative(root, path).replaceAll("\\", "/");
    if (
      [...excluded].some(
        (value) => name === value || name.startsWith(`${value}/`),
      )
    )
      continue;
    if (statSync(path).isDirectory()) yield* walk(path);
    else yield path;
  }
}

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function fail(message) {
  console.error(message);
  process.exit(1);
}
