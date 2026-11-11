// The native window handles the successful-login handoff. Leave every
// validation/error prompt intact, and do not access forms or credentials.
(() => {
  if (window.location.hostname !== 'bbs.nga.cn') return;
  const confirm = window.confirm.bind(window);
  const alert = window.alert.bind(window);
  window.confirm = message => String(message) === '登录成功 是否返回首页' ? false : confirm(message);
  window.alert = message => { if (String(message) !== '登录成功') alert(message); };
})();
