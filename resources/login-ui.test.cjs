const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const source = fs.readFileSync(__dirname + '/login-ui.js', 'utf8');
const shown = [];
const window = {
  location: { hostname: 'bbs.nga.cn' },
  confirm(message) { shown.push(['confirm', message]); return true; },
  alert(message) { shown.push(['alert', message]); }
};
vm.runInNewContext(source, { window });
assert.equal(window.confirm('登录成功 是否返回首页'), false);
window.alert('登录成功');
assert.deepEqual(shown, [], 'successful login still asks for a redundant confirmation');
assert.equal(window.confirm('账号验证失败，是否重试？'), true);
window.alert('密码错误');
assert.deepEqual(shown, [['confirm', '账号验证失败，是否重试？'], ['alert', '密码错误']], 'site errors were suppressed');
console.log('Login success prompt handling: passed');
