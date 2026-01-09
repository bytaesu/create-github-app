#!/usr/bin/env node
const { spawn } = require("node:child_process");
const path = require("node:path");

const BIN_NAME = process.platform === "win32" ? "create-github-app.exe" : "create-github-app";
const BIN_PATH = path.join(__dirname, BIN_NAME);

const child = spawn(BIN_PATH, process.argv.slice(2), { stdio: "inherit" });

child.on("close", (code) => {
  process.exit(code);
});
