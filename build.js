import * as esbuild from "esbuild";
import fs from "fs";
import path from "path";

const DIR = "web/static";
const OUT = `${DIR}/dist`;
const ROOT = process.cwd();

const isWatch = process.argv.includes("--watch");

const highlightConfig = {
  entryPoints: [path.join(ROOT, "scripts/highlight-entry.js")],
  bundle: true,
  minify: true,
  format: "esm",
  target: "es2020",
  outfile: path.join(ROOT, DIR, "chat/highlight.bundle.js"),
  banner: { js: "/*! highlight.js 11.11.1 | BSD-3-Clause | https://highlightjs.org/ */" },
  legalComments: "none",
};

/** @type {esbuild.BuildOptions} */
const jsConfig = {
  entryPoints: {
    admin: path.join(ROOT, DIR, "admin/app.js"),
    account: path.join(ROOT, DIR, "account/app.js"),
  },
  bundle: true,
  minify: true,
  format: "esm",
  target: "es2020",
  outdir: OUT,
  entryNames: "[name].[hash]",
  metafile: true,
};

/** @type {esbuild.BuildOptions} */
const cssEntry = [
  "tokens.css",
  "base.css",
  "components.css",
  "charts.css",
  "layout.css",
  "chat.css",
].map((file) => `@import "./${file}";`).join("\n");

const cssConfig = {
  stdin: {
    contents: cssEntry,
    loader: "css",
    resolveDir: path.join(ROOT, DIR, "css"),
    sourcefile: `${DIR}/css/bundle.css`,
  },
  bundle: true,
  minify: true,
  loader: { ".css": "css" },
  outfile: `${OUT}/style.css`,
  metafile: true,
};

if (isWatch) {
  const highlightCtx = await esbuild.context({ ...highlightConfig, logLevel: "info" });
  await highlightCtx.watch();
  console.log("Watching syntax highlighter...");
  const ctx = await esbuild.context({ ...jsConfig, logLevel: "info" });
  await ctx.watch();
  console.log("Watching JS...");
  const cssCtx = await esbuild.context({ ...cssConfig, logLevel: "info" });
  await cssCtx.watch();
  console.log("Watching CSS...");
  // Keep alive
  process.stdin.resume();
} else {
  fs.rmSync(OUT, { recursive: true, force: true });
  fs.mkdirSync(OUT, { recursive: true });

  await esbuild.build(highlightConfig);
  const jsResult = await esbuild.build(jsConfig);
  const cssResult = await esbuild.build(cssConfig);

  // Write manifest so Go templates can use hashed filenames
  const manifest = {
    adminJS: Object.keys(jsResult.metafile.outputs).find((k) => k.includes("admin.")),
    accountJS: Object.keys(jsResult.metafile.outputs).find((k) => k.includes("account.")),
    css: Object.keys(cssResult.metafile.outputs)[0],
  };

  // Simplify to just the filename
  for (const [key, path] of Object.entries(manifest)) {
    manifest[key] = path ? path.replace(/^.*[/\\]/, "") : path;
  }

  fs.writeFileSync(`${OUT}/manifest.json`, JSON.stringify(manifest, null, 2));

  console.log(`Built: ${manifest.adminJS}, ${manifest.accountJS}, ${manifest.css}`);
}
