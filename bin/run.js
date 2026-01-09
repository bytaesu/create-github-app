#!/usr/bin/env node
const { spawn, spawnSync } = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");

const BIN_NAME = process.platform === "win32" ? "create-github-app.exe" : "create-github-app";
const BIN_PATH = path.join(__dirname, BIN_NAME);

function run() {
	const child = spawn(BIN_PATH, process.argv.slice(2), { stdio: "inherit" });
	child.on("close", (code) => process.exit(code));
}

if (!fs.existsSync(BIN_PATH)) {
	const install = spawnSync("node", [path.join(__dirname, "install.js")], { stdio: "inherit" });
	if (install.status !== 0) {
		process.exit(1);
	}
}

run();
