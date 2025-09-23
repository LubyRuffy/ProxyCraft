# CA

本目录代码主要用于生成和安装CA证书到系统证书库，用于支持https的MITM代理模式。

## 支持平台

- macOS：使用`security add-trusted-cert`写入系统钥匙串。
- Linux：在支持`update-ca-certificates`或`update-ca-trust`的发行版上写入系统根证书目录，并刷新信任列表。
- Windows：通过`certutil`将证书导入到本地计算机的`Root`存储。

安装/卸载系统根证书需要管理员或root权限，运行失败时请确认使用了具有足够权限的终端。
