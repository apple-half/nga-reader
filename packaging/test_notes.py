import json
from pathlib import Path
import re
import tempfile
import unittest
from unittest.mock import patch

from package import ROOT
import publish


class AppUpdateNotesTests(unittest.TestCase):
    def test_signed_app_notes_are_short_and_readable_by_old_plain_text_clients(self):
        notes = (ROOT / 'packaging' / 'update-notes.txt').read_text(encoding='utf-8')
        self.assertTrue(notes.strip())
        self.assertLessEqual(len(notes), 1200)
        self.assertGreaterEqual(len(re.findall(r'^- ', notes, re.M)), 2)
        for clutter in ['https://', 'http://', '<details', '<summary', '|', '**', '##']:
            self.assertNotIn(clutter, notes)

    def notes_directory(self, folder, notes):
        root = Path(folder)
        (root / 'packaging').mkdir()
        (root / 'packaging' / 'update-notes.txt').write_bytes(notes.encode('utf-8'))
        directory = root / 'dist'
        directory.mkdir()
        for target in publish.TARGETS:
            (directory / ('update-' + target + '.json')).write_text(json.dumps({'notes': notes.replace('\n', '\r\n')}), encoding='utf-8')
        return root, directory

    def test_publisher_rejects_web_notes_in_a_signed_metadata_file(self):
        with tempfile.TemporaryDirectory() as folder:
            root, directory = self.notes_directory(folder, '本次更新\n\n- 修复显示\n')
            (directory / 'update-darwin-arm64.json').write_text(json.dumps({'notes': '## 下载\n| 系统 | 下载 |\n'}), encoding='utf-8')
            with patch.object(publish, 'ROOT', root), patch.object(publish.subprocess, 'run'), self.assertRaisesRegex(RuntimeError, 'App update notes'):
                publish.verify_updates(directory)

    def test_publisher_accepts_matching_compact_notes_with_platform_line_endings(self):
        with tempfile.TemporaryDirectory() as folder:
            root, directory = self.notes_directory(folder, '本次更新\n\n- 修复显示\n')
            with patch.object(publish, 'ROOT', root), patch.object(publish.subprocess, 'run') as verify:
                publish.verify_updates(directory)
                verify.assert_called_once()


if __name__ == '__main__':
    unittest.main()
