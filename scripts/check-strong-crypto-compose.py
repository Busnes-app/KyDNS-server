#!/usr/bin/env python3
"""Prove the profile overlay changes only GODEBUG, using synthetic configuration."""
import json
import os
from pathlib import Path
import subprocess

root = Path(__file__).resolve().parent.parent
env = dict(os.environ, KYDNS_IP="192.0.2.53", KYDNS_PARENT_IF="eth0",
           KYDNS_SUBNET="192.0.2.0/24", KYDNS_GATEWAY="192.0.2.1",
           KYDNS_DNS="192.0.2.2", KYDNS_CONFIG="./kydns.example.yaml")
for name in ("COMPOSE_FILE", "COMPOSE_PROFILES", "COMPOSE_ENV_FILES"):
    env.pop(name, None)
for extra in ([], ["docker-compose.lan-dns.yml"]):
    def config(files):
        command = ["docker", "compose", "--env-file", "/dev/null", "-p", "kydns-profile-check"]
        for file in files:
            command += ["-f", file]
        return json.loads(subprocess.check_output(command + ["config", "--format", "json"],
                         cwd=root, env=env, text=True))
    files = ["docker-compose.yml"] + extra
    before = config(files)
    after = config(files + ["docker-compose.strong-crypto.yml"])
    assert after["services"]["kydns"]["environment"].pop("GODEBUG") == "fips140=on"
    assert after == before, "profile changed configuration beyond GODEBUG"
print("Both Compose chains preserve networks, data, DNS, credentials and build selection.")
