"""Create and verify portable archives without an installer or third-party Python dependencies."""
from pathlib import Path, PurePosixPath
import argparse
import hashlib
import json
import plistlib
import re
import stat
import struct
import zipfile

ROOT = Path(__file__).resolve().parent.parent
CONFIG = json.loads((ROOT / 'mygo.json').read_text(encoding='utf-8'))
VERSION = CONFIG['version']
TARGETS = {
    'windows-amd64': 'Windows-x64-portable',
    'darwin-arm64': 'macOS-AppleSilicon-portable',
    'darwin-amd64': 'macOS-Intel-portable',
}


def asset_name(target):
    return f'NGAReader-{VERSION}-{TARGETS[target]}.zip'


def check_version(tag=None):
    version = re.search(r'^const appVersion = "([^"]+)"', (ROOT / 'main.go').read_text(encoding='utf-8'), re.M)
    if not version or version[1] != VERSION or (tag and tag != 'v' + VERSION):
        raise ValueError('App, packaging configuration and release tag versions must match')


def check_binary(data, target):
    if target == 'windows-amd64':
        if len(data) < 64 or data[:2] != b'MZ':
            raise ValueError('Missing Windows executable')
        offset = struct.unpack_from('<I', data, 60)[0]
        if offset + 6 > len(data) or data[offset:offset + 4] != b'PE\0\0' or struct.unpack_from('<H', data, offset + 4)[0] != 0x8664:
            raise ValueError('Windows archive must contain an x64 PE executable')
    else:
        expected = 0x0100000c if target == 'darwin-arm64' else 0x01000007
        if len(data) < 32 or data[:4] != bytes.fromhex('cffaedfe') or struct.unpack_from('<I', data, 4)[0] != expected:
            raise ValueError('Mac executable architecture does not match the archive')


def safe_member(name, mode):
    parts = PurePosixPath(name).parts
    if '\\' in name or name.startswith('/') or '..' in parts or stat.S_ISLNK(mode):
        raise ValueError('Unsafe archive path: ' + name)
    if any(p.lower() in {'data', 'cache', 'logs', 'runtime', '.git', '__pycache__'} for p in parts) or PurePosixPath(name).name.lower() in {'drafts.json', 'settings.json', 'session.enc'}:
        raise ValueError('Local data must not ship: ' + name)
    if name.lower().endswith(('.enc', '.log', '.jsonl', '.dmg', '.pkg', '.msi')) or PurePosixPath(name).name.lower().startswith(('setup', 'uninstall', 'install.')):
        raise ValueError('Unexpected private data or installer: ' + name)


def verify_archive(path, target):
    with zipfile.ZipFile(path) as archive:
        if archive.testzip() is not None:
            raise ValueError('ZIP CRC check failed')
        names = archive.namelist()
        if len(names) != len(set(names)):
            raise ValueError('Duplicate ZIP entries')
        for info in archive.infolist():
            safe_member(info.filename, info.external_attr >> 16)
        if target == 'windows-amd64':
            executable = 'NGAReader/NGAReader.exe'
            license_file = 'NGAReader/LICENSE'
            third_party = 'NGAReader/LICENSES/'
            if not all(n.startswith('NGAReader/') for n in names):
                raise ValueError('Unexpected files outside the portable Windows folder')
        else:
            executable = 'NGAReader.app/Contents/MacOS/NGAReader'
            license_file = 'NGAReader.app/Contents/Resources/LICENSE'
            third_party = 'NGAReader.app/Contents/Resources/LICENSES/'
            if not all(n.startswith('NGAReader.app/') or n == 'README.zh-CN.txt' for n in names):
                raise ValueError('Unexpected files outside the Mac bundle')
            plist = plistlib.loads(archive.read('NGAReader.app/Contents/Info.plist'))
            if plist.get('CFBundleExecutable') != 'NGAReader' or plist.get('CFBundleIdentifier') != CONFIG['identifier'] or plist.get('CFBundleShortVersionString') != VERSION or plist.get('CFBundlePackageType') != 'APPL':
                raise ValueError('Invalid app-bundle metadata')
            info = archive.getinfo(executable)
            if info.create_system != 3 or (info.external_attr >> 16) & 0o111 == 0:
                raise ValueError('Mac executable permission is missing')
        check_binary(archive.read(executable), target)
        if b'MIT License' not in archive.read(license_file) or not any(n.startswith(third_party) for n in names):
            raise ValueError('Required licenses missing')
        return {'target': target, 'file': Path(path).name, 'members': len(names), 'bytes': Path(path).stat().st_size, 'sha256': hashlib.sha256(Path(path).read_bytes()).hexdigest()}


def readme(target):
    start = f'NGA Reader {VERSION} — 免安装便携预发布\n\n'
    if target == 'windows-amd64':
        start += '解压整个 NGAReader 文件夹，双击 NGAReader.exe。无需安装或管理员权限；文件夹应可写。更新前退出旧版，保留原来的 data 文件夹。\n设置与加密会话保存在 EXE 同级 data 目录。官方登录和 App 内图片预览需要已有 WebView2 Runtime。\n'
    else:
        chip = 'Apple 芯片（M 系列）' if target == 'darwin-arm64' else 'Intel 芯片'
        start += f'适用于 macOS 12 或更高版本，{chip}。解压后打开 NGAReader.app，无需安装向导，也不强制复制到 Applications。\n设置与加密会话保存在 ~/Library/Application Support/NGAReader，不写入 .app 包内。\n本包仅有 ad-hoc 完整性签名，没有 Apple Developer ID 签名或 Apple 公证；macOS 可能阻止运行。请先核实来源与校验值，不要关闭系统防护。\n'
    return start + '\n图片默认折叠，点击“展示图片”才加载；软件侧栏内置使用说明。\n\n早期 Windows 0.1.2 曾被360检出，0.1.2/0.1.3复核未通过且未提供具体判定依据。本版尚无360放行结论。功能测试及源码公开不构成恶意软件鉴定。若被拦截，保留隔离并向厂商核实。\n\n源码与问题反馈：https://github.com/apple-half/nga-reader\n'


def create_archive(target, build_root, output):
    check_version()
    source = Path(build_root) / target
    app = source / 'NGAReader.app'
    if target.startswith('darwin-'):
        source = app
        prefix = Path('NGAReader.app')
    else:
        prefix = Path('NGAReader')
    if not source.is_dir():
        raise ValueError('Build output is missing: ' + str(source))
    output = Path(output)
    output.mkdir(parents=True, exist_ok=True)
    path = output / asset_name(target)
    with zipfile.ZipFile(path, 'w', zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
        for item in sorted(source.rglob('*')):
            if item.is_symlink():
                raise ValueError('Symbolic links are not expected in this bundle')
            if not item.is_file():
                continue
            name = (prefix / item.relative_to(source)).as_posix()
            mode = stat.S_IFREG | (0o755 if name.endswith('/Contents/MacOS/NGAReader') else 0o644)
            safe_member(name, mode)
            info = zipfile.ZipInfo(name)
            info.create_system = 3
            info.external_attr = mode << 16
            info.compress_type = zipfile.ZIP_DEFLATED
            archive.writestr(info, item.read_bytes())
        name = 'NGAReader/README.zh-CN.txt' if target.startswith('windows-') else 'README.zh-CN.txt'
        info = zipfile.ZipInfo(name)
        info.create_system = 3
        info.external_attr = (stat.S_IFREG | 0o644) << 16
        info.compress_type = zipfile.ZIP_DEFLATED
        archive.writestr(info, readme(target).encode('utf-8-sig'))
    return verify_archive(path, target)


def checksums(output):
    output = Path(output)
    expected = {asset_name(t) for t in TARGETS}
    if {p.name for p in output.glob('*.zip')} != expected:
        raise ValueError('All three platform archives are required before publishing')
    results = [verify_archive(output / asset_name(t), t) for t in TARGETS]
    (output / 'SHA256SUMS.txt').write_text(''.join(r['sha256'] + '  ' + r['file'] + '\n' for r in sorted(results, key=lambda x: x['file'])), encoding='utf-8')
    return results


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--target', choices=TARGETS)
    parser.add_argument('--build-root', default='build')
    parser.add_argument('--output', default='dist')
    parser.add_argument('--verify')
    parser.add_argument('--checksums', action='store_true')
    parser.add_argument('--check-version')
    args = parser.parse_args()
    if args.check_version:
        check_version(args.check_version)
        print('Release version matches ' + args.check_version)
    elif args.checksums:
        print(json.dumps(checksums(args.output)))
    elif args.verify:
        if not args.target: parser.error('--target is required for verification')
        print(json.dumps(verify_archive(args.verify, args.target)))
    else:
        if not args.target: parser.error('--target is required')
        print(json.dumps(create_archive(args.target, args.build_root, args.output)))
