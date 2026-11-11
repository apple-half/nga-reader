"""Copy public, already verified update metadata to the API-independent index.

The caller must verify the complete signed directory with update-release first.
This module rechecks the published asset digests and commits all platforms in one
non-forced Git ref update. It never signs files or edits a Release.
"""
import base64
import hashlib
import json
from pathlib import Path
import re
import tempfile

from package import VERSION, TARGETS


def version_tuple(value):
    if not isinstance(value, str) or not re.fullmatch(r'[0-9]+\.[0-9]+\.[0-9]+', value):
        raise RuntimeError('Invalid version in update index')
    return tuple(int(part) for part in value.split('.'))


def api(gh, endpoint, payload=None, method='POST'):
    if payload is None:
        return json.loads(gh('api', endpoint))
    with tempfile.TemporaryDirectory(prefix='nga-public-feed-') as folder:
        path = Path(folder) / 'request.json'
        path.write_text(json.dumps(payload, ensure_ascii=False), encoding='utf-8')
        return json.loads(gh('api', '--method', method, endpoint, '--input', str(path)))


def publish_feeds(repo, tag, directory, current, gh):
    if repo != 'apple-half/nga-reader' or tag != 'v' + VERSION or current.get('tag_name') != tag or current.get('draft') is not False:
        raise RuntimeError('Update index requires the matching published release')
    assets = {asset['name']: asset for asset in current['assets']}
    entries = []
    # Check every platform before creating any repository objects.
    for target in TARGETS:
        name = 'update-' + target + '.json'
        data = (directory / name).read_bytes()
        asset = assets.get(name, {})
        if asset.get('state') != 'uploaded' or asset.get('size') != len(data) or asset.get('digest') != 'sha256:' + hashlib.sha256(data).hexdigest():
            raise RuntimeError('Published metadata size/digest differs: ' + name)
        manifest = json.loads(data)
        expected_url = f'https://github.com/{repo}/releases/download/{tag}/nga-reader-{VERSION}-{target}.tar.gz'
        if manifest.get('version') != VERSION or manifest.get('target') != target or manifest.get('url') != expected_url:
            raise RuntimeError('Published metadata identity differs: ' + name)
        wrapper = {'manifest': manifest, 'prerelease': bool(current.get('prerelease'))}
        entries.append({'path': 'updates/' + name, 'mode': '100644', 'type': 'blob', 'content': json.dumps(wrapper, ensure_ascii=False, indent=2) + '\n'})
    head = api(gh, f'repos/{repo}/git/ref/heads/main')['object']['sha']
    changed = []
    for entry in entries:
        response = api(gh, f'repos/{repo}/contents/{entry["path"]}?ref={head}')
        previous = json.loads(base64.b64decode(response['content']))
        wrapper = json.loads(entry['content'])
        previous_version = version_tuple(previous['manifest']['version'])
        if previous_version > version_tuple(VERSION):
            raise RuntimeError('Refusing to downgrade a newer update index')
        if previous_version == version_tuple(VERSION) and previous['manifest'] != wrapper['manifest']:
            raise RuntimeError('Conflicting metadata for the same version')
        if previous != wrapper:
            changed.append(entry)
    if not changed:
        print('Public update indexes already match ' + tag)
        return False
    tree = api(gh, f'repos/{repo}/git/commits/{head}')['tree']['sha']
    new_tree = api(gh, f'repos/{repo}/git/trees', {'base_tree': tree, 'tree': changed})['sha']
    commit = api(gh, f'repos/{repo}/git/commits', {'message': f'Publish signed update indexes for {tag}', 'tree': new_tree, 'parents': [head]})['sha']
    # A concurrent main update must fail visibly. Never force away other work.
    api(gh, f'repos/{repo}/git/refs/heads/main', {'sha': commit, 'force': False}, method='PATCH')
    print('Published verified update indexes for ' + tag)
    return True
