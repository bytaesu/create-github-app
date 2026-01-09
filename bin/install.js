#!/usr/bin/env node
const fs = require("node:fs");
const path = require("node:path");
const https = require("node:https");

const REPO = "bytaesu/create-github-app";
const BIN_NAME =
	process.platform === "win32" ? "create-github-app.exe" : "create-github-app";
const BIN_PATH = path.join(__dirname, BIN_NAME);

const PLATFORM_MAP = {
	darwin: "darwin",
	linux: "linux",
	win32: "windows",
};

const ARCH_MAP = {
	x64: "amd64",
	arm64: "arm64",
};

async function getLatestRelease() {
	return new Promise((resolve, reject) => {
		https
			.get(
				`https://api.github.com/repos/${REPO}/releases/latest`,
				{ headers: { "User-Agent": "create-github-app" } },
				(res) => {
					let data = "";
					res.on("data", (chunk) => (data += chunk));
					res.on("end", () => {
						try {
							resolve(JSON.parse(data));
						} catch (e) {
							reject(e);
						}
					});
				},
			)
			.on("error", reject);
	});
}

async function download(url, dest) {
	return new Promise((resolve, reject) => {
		const file = fs.createWriteStream(dest);
		https
			.get(url, { headers: { "User-Agent": "create-github-app" } }, (res) => {
				if (res.statusCode === 302 || res.statusCode === 301) {
					download(res.headers.location, dest).then(resolve).catch(reject);
					return;
				}
				res.pipe(file);
				file.on("finish", () => {
					file.close();
					resolve();
				});
			})
			.on("error", (err) => {
				fs.unlink(dest, () => {});
				reject(err);
			});
	});
}

async function main() {
	const platform = PLATFORM_MAP[process.platform];
	const arch = ARCH_MAP[process.arch];

	if (!platform || !arch) {
		console.error(`Unsupported platform: ${process.platform}-${process.arch}`);
		process.exit(1);
	}

	const assetName = `create-github-app-${platform}-${arch}${platform === "windows" ? ".exe" : ""}`;

	try {
		const release = await getLatestRelease();
		const asset = release.assets?.find((a) => a.name === assetName);

		if (!asset) {
			console.error(`Binary not found for ${platform}-${arch}`);
			console.error(
				"Available assets:",
				release.assets?.map((a) => a.name).join(", "),
			);
			process.exit(1);
		}

		console.log(`Downloading ${assetName}...`);
		await download(asset.browser_download_url, BIN_PATH);
		fs.chmodSync(BIN_PATH, 0o755);
		console.log("Done.");
	} catch (err) {
		console.error("Failed to install:", err.message);
		process.exit(1);
	}
}

main().catch(() => process.exit(1));
