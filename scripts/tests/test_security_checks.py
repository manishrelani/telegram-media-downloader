import importlib.util
from pathlib import Path
import unittest
ROOT=Path(__file__).resolve().parents[2]
spec=importlib.util.spec_from_file_location('security_checks', ROOT/'scripts/security_checks.py')
module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)

class SecurityChecksTests(unittest.TestCase):
    def test_osv_rejects_advisories_and_incomplete_responses(self):
        for payload in [{'results':[{'vulns':[{'id':'TEST-1'}]}]}, {'results':[]}, {}, {'results':[{'error':'unavailable'}]}]:
            with self.subTest(payload=payload), self.assertRaises(RuntimeError):
                module.check_osv(payload, 1)
    def test_clean_osv_response(self):
        module.check_osv({'results':[{}, {}]}, 2)
