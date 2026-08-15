import { createHash } from "node:crypto";
import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import assert from "node:assert/strict";
import test from "node:test";
import { projectRoot } from "../helpers/runner.ts";

test("repository ships the approved visual identity", () => {
  const banner = readFileSync(join(projectRoot, "assets", "banner.png"));
  const digest = createHash("sha256").update(banner).digest("hex");
  assert.equal(
    digest,
    "c9480cef4d7a69c4169e18fce27d9a7f7f305988c6e0255bde1c8e56db602cc6",
  );
  assert.match(
    readFileSync(join(projectRoot, "README.md"), "utf8"),
    /^# CitadelDTL$/m,
  );
});

test("documentation contract contains seven linked guides and rich diagrams", () => {
  const docs = readdirSync(join(projectRoot, "docs"))
    .filter((name) => name.endsWith(".md"))
    .sort();
  assert.deepEqual(docs, [
    "01-arquitectura.md",
    "02-modelo-economico.md",
    "03-seguridad-operativa.md",
    "04-api-y-sdk.md",
    "05-operacion.md",
    "06-gobernanza.md",
    "07-observabilidad.md",
  ]);
  const readme = readFileSync(join(projectRoot, "README.md"), "utf8");
  for (const name of docs)
    assert.match(readme, new RegExp(`docs/${name.replace(".", "\\.")}`));
  const diagrams = [
    readme,
    readFileSync(join(projectRoot, "SECURITY.md"), "utf8"),
  ]
    .concat(
      docs.map((name) => readFileSync(join(projectRoot, "docs", name), "utf8")),
    )
    .join("\n")
    .match(/```mermaid\b/g);
  assert.ok((diagrams?.length ?? 0) >= 18);
});

test("automation validates candidates, production, tags and releases", () => {
  const ci = readFileSync(
    join(projectRoot, ".github", "workflows", "ci.yml"),
    "utf8",
  );
  const integrity = readFileSync(
    join(projectRoot, ".github", "workflows", "release-integrity.yml"),
    "utf8",
  );
  assert.match(ci, /os: \[ubuntu-latest, windows-latest\]/);
  assert.match(ci, /actions\/checkout@v7/);
  assert.match(ci, /actions\/setup-go@v7/);
  assert.match(ci, /actions\/setup-node@v7/);
  assert.match(integrity, /branches: \[main, production\]/);
  assert.match(integrity, /types: \[published\]/);
  assert.match(integrity, /cat-file -t/);
});
