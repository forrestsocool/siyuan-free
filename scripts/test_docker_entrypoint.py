import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[1]
SHELL = shutil.which("sh")
if not SHELL and os.name == "nt" and shutil.which("git"):
    candidate = Path(shutil.which("git")).resolve().parents[1] / "bin" / "sh.exe"
    if candidate.exists():
        SHELL = str(candidate)


@unittest.skipUnless(SHELL, "A POSIX shell is required")
class DockerEntrypointTests(unittest.TestCase):
    def run_entrypoint(self, *args, workspace=None):
        with tempfile.TemporaryDirectory() as directory:
            bin_dir = Path(directory)
            stubs = {
                "getent": 'case "$1" in group) echo "siyuan:x:100:sentinel";; passwd) echo "siyuan:x:1000:100::/home/siyuan:/bin/sh";; esac',
                "chown": "exit 0",
                "su-exec": 'printf "ARG:%s\\n" "$@"',
            }
            for name, body in stubs.items():
                stub = bin_dir / name
                stub.write_text("#!/bin/sh\n" + body + "\n", encoding="utf-8")
                stub.chmod(0o755)
            env = dict(os.environ, PATH=str(bin_dir) + os.pathsep + os.environ["PATH"], PUID="1000", PGID="100")
            shell_bin = bin_dir.as_posix()
            if os.name == "nt":
                shell_bin = "/" + shell_bin[0].lower() + shell_bin[2:]
            env["SIYUAN_TEST_BIN"] = shell_bin
            env["SIYUAN_TEST_ENTRYPOINT"] = (ROOT / "kernel" / "entrypoint.sh").as_posix()
            env.pop("SIYUAN_WORKSPACE_PATH", None)
            if workspace is not None:
                env["SIYUAN_WORKSPACE_PATH"] = workspace
            result = subprocess.run([SHELL, "-c", 'export PATH="$SIYUAN_TEST_BIN:$PATH"; . "$SIYUAN_TEST_ENTRYPOINT"', "entrypoint", *args],
                                    env=env, capture_output=True, text=True, timeout=15)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            return [line.removeprefix("ARG:") for line in result.stdout.splitlines() if line.startswith("ARG:")]

    def test_legacy_compose_flags_start_server(self):
        self.assertEqual(self.run_entrypoint("--workspace=/siyuan/workspace/", "--accessAuthCode=example"),
                         ["1000:100", "/opt/siyuan/kernel", "--workspace=/siyuan/workspace/", "serve",
                          "--workspace=/siyuan/workspace/", "--accessAuthCode=example"])

    def test_explicit_subcommands_are_preserved(self):
        for command in ("serve", "version"):
            with self.subTest(command=command):
                self.assertEqual(self.run_entrypoint(command)[-1], command)

    def test_no_arguments_or_legacy_binary_start_server(self):
        self.assertEqual(self.run_entrypoint()[-1], "serve")
        self.assertEqual(self.run_entrypoint("/opt/siyuan/kernel")[-1], "serve")

    def test_arguments_with_spaces_are_preserved(self):
        self.assertEqual(self.run_entrypoint("--accessAuthCode=two words", workspace="/workspace with spaces")[-3:],
                         ["--workspace=/workspace with spaces", "serve", "--accessAuthCode=two words"])


if __name__ == "__main__":
    unittest.main()
