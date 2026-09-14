import unittest

from check_macos_target import check_target


def commands(version="12.0", platform="1"):
    return f"Load command 0\n      cmd LC_BUILD_VERSION\n platform {platform}\n    minos {version}\n"


class TargetTest(unittest.TestCase):
    def test_native_and_universal(self):
        self.assertEqual(check_target(commands()), ["12.0"])
        self.assertEqual(check_target(commands() + commands("12.0.0")), ["12.0", "12.0.0"])

    def test_rejects_mismatch_missing_or_wrong_platform(self):
        for output in (commands("15.0"), commands("11.0"), "", commands(platform="2"),
                       commands() + commands("15.0"), commands().replace("minos", "sdk")):
            with self.subTest(output=output), self.assertRaises(ValueError):
                check_target(output)

    def test_legacy_load_command(self):
        self.assertEqual(check_target(
            "Load command 0\n cmd LC_VERSION_MIN_MACOSX\n version 12.0\n"
        ), ["12.0"])


if __name__ == "__main__":
    unittest.main()
