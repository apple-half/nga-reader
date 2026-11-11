"""Publish only a complete verified asset set, using GitHub Actions' scoped token."""
from pathlib import Path
import hashlib
import json
import os
import subprocess

from package import ROOT, VERSION, TARGETS, asset_name, check_version, checksums


def gh(*args):
    result = subprocess.run(['gh', *args], check=True, capture_output=True, text=True)
    return result.stdout


def release(repo, tag, missing_ok=False):
    result = subprocess.run(['gh', 'api', f'repos/{repo}/releases/tags/{tag}'], capture_output=True, text=True)
    if result.returncode:
        if missing_ok and 'HTTP 404' in result.stderr:
            return None
        raise RuntimeError(result.stderr)
    return json.loads(result.stdout)


def main():
    repo = os.environ['GITHUB_REPOSITORY']
    tag = os.environ['RELEASE_TAG']
    check_version(tag)
    directory = ROOT / 'dist'
    checksums(directory)
    paths = [directory / asset_name(target) for target in TARGETS] + [directory / 'SHA256SUMS.txt']
    current = release(repo, tag, missing_ok=True)
    if current and not current['draft']:
        raise RuntimeError('Refusing to change an already published release')
    if current is None:
        gh('release', 'create', tag, '--repo', repo, '--verify-tag', '--draft', '--prerelease', '--title', f'NGA Reader {VERSION} · 免安装便携版', '--notes-file', str(ROOT / 'packaging' / 'release-notes.md'))
    gh('release', 'upload', tag, '--repo', repo, '--clobber', *(str(path) for path in paths))
    uploaded = release(repo, tag)
    assets = {asset['name']: asset for asset in uploaded['assets']}
    if set(assets) != {path.name for path in paths}:
        raise RuntimeError('Draft assets do not match the expected release files')
    for path in paths:
        asset = assets[path.name]
        expected = 'sha256:' + hashlib.sha256(path.read_bytes()).hexdigest()
        if asset['state'] != 'uploaded' or asset['size'] != path.stat().st_size or asset.get('digest') != expected:
            raise RuntimeError('Uploaded asset size/digest differs: ' + path.name)
    # Keep the release as a draft until every platform and checksum file agrees.
    gh('release', 'edit', tag, '--repo', repo, '--prerelease', '--draft=false')
    final = release(repo, tag)
    if final['draft'] or not final['prerelease']:
        raise RuntimeError('Release did not reach the expected public prerelease state')
    print(final['html_url'])


if __name__ == '__main__':
    main()
