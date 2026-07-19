// 设备标识:首次访问生成 UUID 存 localStorage,作为无登录体系下的用户身份。
// 服务端按它区分每个人的播放历史与收听统计。

const KEY = "tutu:device";

export function deviceId(): string {
  try {
    let id = localStorage.getItem(KEY);
    if (!id) {
      id = crypto.randomUUID();
      localStorage.setItem(KEY, id);
    }
    return id;
  } catch {
    return "anonymous"; // 隐私模式等存储不可用,退化为共享身份
  }
}
