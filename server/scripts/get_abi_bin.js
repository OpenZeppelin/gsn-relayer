#!/usr/bin/env node

const { readFileSync, writeFileSync } = require("fs");

const contract = process.argv[2];
console.log(`Extracting ABI for ${contract}`);

const artifact = JSON.parse(readFileSync(`../build/contracts/${contract}.json`));

writeFileSync(`../build/contracts/${contract}.abi`, JSON.stringify(artifact.abi));
if (artifact.bytecode !== '0x') {
  writeFileSync(`../build/contracts/${contract}.bin`, JSON.stringify(artifact.bin));
}
