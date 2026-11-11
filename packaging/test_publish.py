import hashlib
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import publish


class PublishTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        output = self.root / 'dist'
        output.mkdir()
        names = [publish.asset_name(t) for t in publish.TARGETS] + ['SHA256SUMS.txt']
        self.assets = []
        for name in names:
            data = ('synthetic fixture ' + name).encode()
            (output / name).write_bytes(data)
            self.assets.append({'name': name, 'state': 'uploaded', 'size': len(data), 'digest': 'sha256:' + hashlib.sha256(data).hexdigest()})
        self.calls = []
        self.state = {'draft': True, 'prerelease': False}

    def gh(self, *args):
        self.calls.append(args)
        if args[:2] == ('release', 'edit'):
            self.state['draft'] = '--draft=false' not in args
            if '--prerelease' in args:
                self.state['prerelease'] = True

    def release(self, *args, **kwargs):
        return {**self.state, 'assets': self.assets, 'html_url': 'https://github.com/example/reader/releases/tag/v' + publish.VERSION}

    def run_publish(self):
        with patch.dict(os.environ, {'GITHUB_REPOSITORY': 'example/reader', 'RELEASE_TAG': 'v' + publish.VERSION}), patch.object(publish, 'ROOT', self.root), patch.object(publish, 'check_version'), patch.object(publish, 'checksums'), patch.object(publish, 'release', side_effect=self.release), patch.object(publish, 'gh', side_effect=self.gh):
            publish.main()

    def test_reused_stable_draft_is_made_prerelease_before_exposure(self):
        self.run_publish()
        edits = [call for call in self.calls if call[:2] == ('release', 'edit')]
        self.assertEqual(len(edits), 1)
        self.assertIn('--draft=false', edits[0])
        self.assertIn('--prerelease', edits[0])
        self.assertEqual(self.state, {'draft': False, 'prerelease': True})

    def test_wrong_uploaded_digest_leaves_release_as_draft(self):
        self.assets[0]['digest'] = 'sha256:' + '0' * 64
        with self.assertRaisesRegex(RuntimeError, 'digest differs'):
            self.run_publish()
        self.assertTrue(self.state['draft'])
        self.assertFalse(any(call[:2] == ('release', 'edit') for call in self.calls))

    def test_existing_public_release_is_immutable(self):
        self.state['draft'] = False
        with self.assertRaisesRegex(RuntimeError, 'already published'):
            self.run_publish()
        self.assertEqual(self.calls, [])


if __name__ == '__main__':
    unittest.main()
