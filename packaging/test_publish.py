import hashlib
import json
import os
from pathlib import Path
import subprocess
import stat
import tempfile
import unittest
from unittest.mock import patch

import publish
import package


class ArchivePrivacyTests(unittest.TestCase):
    def test_private_draft_and_settings_files_never_ship(self):
        for name in ['NGAReader/drafts.json', 'NGAReader/settings.json', 'NGAReader.app/Contents/Resources/drafts.json']:
            with self.subTest(name=name), self.assertRaisesRegex(ValueError, 'Local data'):
                package.safe_member(name, stat.S_IFREG | 0o644)


class ReleaseLookupTests(unittest.TestCase):
    def test_finds_draft_when_published_tag_endpoint_returns_404(self):
        draft = {'id': 42, 'tag_name': 'v0.1.5', 'draft': True}

        def api(command, **kwargs):
            if '/releases/tags/' in command[-1]:
                return subprocess.CompletedProcess(command, 1, '', 'gh: Not Found (HTTP 404)')
            self.assertIn('--paginate', command)
            self.assertIn('--slurp', command)
            return subprocess.CompletedProcess(command, 0, json.dumps([[{'tag_name': 'v0.1.6'}], [draft]]), '')

        with patch.object(publish.subprocess, 'run', side_effect=api):
            self.assertEqual(publish.release('example/reader', 'v0.1.5'), draft)

    def test_missing_release_is_only_optional_after_successful_lookup(self):
        result = subprocess.CompletedProcess([], 0, '[[]]', '')
        with patch.object(publish.subprocess, 'run', return_value=result):
            self.assertIsNone(publish.release('example/reader', 'v0.1.5', missing_ok=True))
            with self.assertRaisesRegex(RuntimeError, 'not found'):
                publish.release('example/reader', 'v0.1.5')

    def test_lookup_permission_failure_is_not_treated_as_missing_release(self):
        result = subprocess.CompletedProcess([], 1, '', 'gh: Resource not accessible (HTTP 403)')
        with patch.object(publish.subprocess, 'run', return_value=result):
            with self.assertRaisesRegex(RuntimeError, 'HTTP 403'):
                publish.release('example/reader', 'v0.1.5', missing_ok=True)


class PublishTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        output = self.root / 'dist'
        output.mkdir()
        names = [publish.asset_name(t) for t in publish.TARGETS] + publish.update_asset_names() + ['SHA256SUMS.txt']
        self.assets = []
        for name in names:
            data = ('synthetic fixture ' + name).encode()
            (output / name).write_bytes(data)
            self.assets.append({'name': name, 'state': 'uploaded', 'size': len(data), 'digest': 'sha256:' + hashlib.sha256(data).hexdigest()})
        self.calls = []
        self.state = {'draft': True, 'prerelease': False}
        self.verify_error = None
        self.feed_error = None
        self.feed_calls = []

    def gh(self, *args):
        self.calls.append(args)
        if args[:2] == ('release', 'edit'):
            self.state['draft'] = '--draft=false' not in args
            if '--prerelease' in args:
                self.state['prerelease'] = True

    def release(self, *args, **kwargs):
        return {**self.state, 'assets': self.assets, 'html_url': 'https://github.com/example/reader/releases/tag/v' + publish.VERSION}

    def feed(self, repo, tag, directory, current, gh):
        self.assertFalse(current['draft'], 'Index exposed private draft assets')
        self.feed_calls.append((repo, tag, directory))
        if self.feed_error:
            raise self.feed_error

    def run_publish(self, stage_only=False, feed_only=False):
        with patch.dict(os.environ, {'GITHUB_REPOSITORY': 'example/reader', 'RELEASE_TAG': 'v' + publish.VERSION}), patch.object(publish, 'ROOT', self.root), patch.object(publish, 'check_version'), patch.object(publish, 'checksums'), patch.object(publish, 'verify_updates', side_effect=self.verify_error), patch.object(publish, 'release', side_effect=self.release), patch.object(publish, 'gh', side_effect=self.gh), patch.object(publish, 'publish_feeds', side_effect=self.feed):
            publish.main(stage_only=stage_only, feed_only=feed_only)

    def test_reused_stable_draft_is_made_prerelease_before_exposure(self):
        self.run_publish()
        edits = [call for call in self.calls if call[:2] == ('release', 'edit')]
        self.assertEqual(len(edits), 1)
        self.assertIn('--draft=false', edits[0])
        self.assertIn('--prerelease', edits[0])
        self.assertEqual(self.state, {'draft': False, 'prerelease': True})
        self.assertEqual(len(self.feed_calls), 1)

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

    def test_stage_uploads_only_portable_assets_and_stays_draft(self):
        self.assets = [a for a in self.assets if a['name'] not in publish.update_asset_names()]
        self.run_publish(stage_only=True)
        self.assertTrue(self.state['draft'])
        self.assertFalse(any(c[:2] == ('release', 'edit') for c in self.calls))
        self.assertEqual(self.feed_calls, [])

    def test_finalization_requires_offline_signatures_before_upload(self):
        self.verify_error = RuntimeError('invalid update signature')
        with self.assertRaisesRegex(RuntimeError, 'signature'):
            self.run_publish()
        self.assertTrue(self.state['draft'])
        self.assertEqual(self.calls, [])
        self.assertEqual(self.feed_calls, [])

    def test_signed_draft_cannot_be_rebuilt_over_existing_signatures(self):
        with self.assertRaisesRegex(RuntimeError, 'already contains signed'):
            self.run_publish(stage_only=True)
        self.assertEqual(self.calls, [])

    def test_failed_feed_can_be_repaired_without_touching_public_assets(self):
        self.feed_error = RuntimeError('non-fast-forward')
        with self.assertRaisesRegex(RuntimeError, 'non-fast-forward'):
            self.run_publish()
        self.assertFalse(self.state['draft'])
        self.feed_error = None
        self.calls.clear()
        self.run_publish(feed_only=True)
        self.assertEqual(self.calls, [])
        self.assertEqual(len(self.feed_calls), 2)

    def test_feed_repair_still_requires_signature_verification(self):
        self.state['draft'] = False
        self.verify_error = RuntimeError('invalid update signature')
        with self.assertRaisesRegex(RuntimeError, 'signature'):
            self.run_publish(feed_only=True)
        self.assertEqual(self.feed_calls, [])
        self.assertEqual(self.calls, [])


if __name__ == '__main__':
    unittest.main()
