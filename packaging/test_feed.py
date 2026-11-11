import base64
import hashlib
import json
from pathlib import Path
import tempfile
import unittest

import feed


class FeedPublicationTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.directory = Path(self.temp.name)
        self.repo = 'apple-half/nga-reader'
        self.tag = 'v' + feed.VERSION
        self.current = {'tag_name': self.tag, 'draft': False, 'prerelease': True, 'assets': []}
        self.old = {}
        self.calls = []
        for target in feed.TARGETS:
            name = 'update-' + target + '.json'
            manifest = {'version': feed.VERSION, 'target': target, 'url': f'https://github.com/{self.repo}/releases/download/{self.tag}/nga-reader-{feed.VERSION}-{target}.tar.gz'}
            data = json.dumps(manifest).encode()
            (self.directory / name).write_bytes(data)
            self.current['assets'].append({'name': name, 'state': 'uploaded', 'size': len(data), 'digest': 'sha256:' + hashlib.sha256(data).hexdigest()})
            previous = {**manifest, 'version': '0.0.1'}
            self.old[name] = {'manifest': previous, 'prerelease': True}

    def gh(self, *args):
        payload = None
        if '--input' in args:
            payload = json.loads(Path(args[args.index('--input') + 1]).read_text())
        self.calls.append((args, payload))
        endpoint = args[args.index('--input') - 1] if payload is not None else args[-1]
        if '/contents/updates/' in endpoint:
            name = endpoint.split('/updates/')[1].split('?')[0]
            return json.dumps({'content': base64.b64encode(json.dumps(self.old[name]).encode()).decode()})
        if endpoint.endswith('/git/ref/heads/main'):
            return json.dumps({'object': {'sha': 'head-sha'}})
        if endpoint.endswith('/git/commits/head-sha'):
            return json.dumps({'tree': {'sha': 'base-tree'}})
        if endpoint.endswith('/git/trees'):
            return json.dumps({'sha': 'new-tree'})
        if endpoint.endswith('/git/commits'):
            return json.dumps({'sha': 'new-commit'})
        if endpoint.endswith('/git/refs/heads/main'):
            return '{}'
        self.fail('Unexpected API endpoint: ' + endpoint)

    def run_feed(self):
        return feed.publish_feeds(self.repo, self.tag, self.directory, self.current, self.gh)

    def mutations(self):
        return [(args, body) for args, body in self.calls if body is not None]

    def test_three_platform_indexes_are_committed_atomically_after_publication(self):
        self.assertTrue(self.run_feed())
        mutations = self.mutations()
        self.assertEqual(len(mutations), 3)
        tree = mutations[0][1]
        self.assertEqual(tree['base_tree'], 'base-tree')
        self.assertEqual({row['path'] for row in tree['tree']}, {'updates/update-' + t + '.json' for t in feed.TARGETS})
        for row in tree['tree']:
            wrapper = json.loads(row['content'])
            self.assertEqual(wrapper['manifest']['version'], feed.VERSION)
            self.assertTrue(wrapper['prerelease'])
        self.assertEqual(mutations[1][1]['parents'], ['head-sha'])
        self.assertEqual(mutations[2][1], {'sha': 'new-commit', 'force': False})
        self.assertFalse(any('release' in args for args, _ in self.calls))

    def test_draft_or_mismatched_tag_never_changes_feed(self):
        for change in [{'draft': True}, {'tag_name': 'v0.0.2'}]:
            with self.subTest(change=change):
                current = {**self.current, **change}
                with self.assertRaisesRegex(RuntimeError, 'published release'):
                    feed.publish_feeds(self.repo, self.tag, self.directory, current, self.gh)
        self.assertEqual(self.calls, [])

    def test_release_metadata_digest_must_match_the_verified_local_file(self):
        self.current['assets'][0]['digest'] = 'sha256:' + '0' * 64
        with self.assertRaisesRegex(RuntimeError, 'digest'):
            self.run_feed()
        self.assertEqual(self.calls, [])

    def test_repeat_is_idempotent_and_old_release_cannot_downgrade_indexes(self):
        for target in feed.TARGETS:
            name = 'update-' + target + '.json'
            self.old[name] = {'manifest': json.loads((self.directory / name).read_text()), 'prerelease': True}
        self.assertFalse(self.run_feed())
        self.assertEqual(self.mutations(), [])
        self.old['update-windows-amd64.json']['manifest']['version'] = '999.0.0'
        with self.assertRaisesRegex(RuntimeError, 'downgrade'):
            self.run_feed()
        self.assertEqual(self.mutations(), [])

    def test_conflicting_same_version_manifest_is_rejected(self):
        target = next(iter(feed.TARGETS))
        name = 'update-' + target + '.json'
        manifest = json.loads((self.directory / name).read_text())
        self.old[name] = {'manifest': {**manifest, 'url': 'https://example.invalid/changed'}, 'prerelease': True}
        with self.assertRaisesRegex(RuntimeError, 'same version'):
            self.run_feed()
        self.assertEqual(self.mutations(), [])

    def test_ref_conflict_is_reported_without_force_or_asset_mutation(self):
        def conflict(*args):
            if '/git/refs/heads/main' in ' '.join(args):
                raise RuntimeError('non-fast-forward')
            return self.gh(*args)
        with self.assertRaisesRegex(RuntimeError, 'non-fast-forward'):
            feed.publish_feeds(self.repo, self.tag, self.directory, self.current, conflict)
        self.assertEqual(len(self.mutations()), 2)


if __name__ == '__main__':
    unittest.main()
