<div align="center">
    <h1> HackSpeed - 黑客喵 </h1>
    <p>🐱 高性能节点脚本偷窃后端 🚀</p>
    <p><a href="https://github.com/AirportR/miaospeed/blob/master/README-EN.md">English</a> &nbsp;&nbsp; 简体中文</p>
    <a href="https://koipy.gitbook.io/koipy/doc/miaospeed-hou-duan"><img src="https://img.shields.io/static/v1?message=doc&color=blue&logo=go&label=miaospeed"></a> 
    <img src="https://img.shields.io/github/license/AirportR/miaospeed">
    <a href="https://github.com/AirportR/miaospeed/issues"><img src="https://img.shields.io/badge/contributions-welcome-brightgreen.svg?style=flat"></a>
    <br>
    <img alt="Docker Pulls" src="https://img.shields.io/docker/pulls/airportr/miaospeed">
    <img alt="Docker Image Size" src="https://img.shields.io/docker/image-size/airportr/miaospeed">
    <br>
    <a href="https://github.com/AirportR/miaospeed/"><img src="https://img.shields.io/github/stars/AirportR/miaospeed?style=social"></a>
	<br>
	<br>
</div>

---

## 简介

⚠️ 不太道德了有点，修改AirportR的二次miaospeed变为三次制作

⚠️ 偷窃miaoko/koipy节点的好帮手

## 编译需求

由于 miaospeed 最初与闭源项目 miaoko 搭配使用，因此部分证书与脚本未开源。您需要补齐以下文件以成功编译:

1. `./utils/embeded/BUILDTOKEN.key`: 这是 `编译TOKEN`，用于给 miaospeed 的 单次测试请求 结构体签名，避免客户端以非标准方式使用 miaospeed 导致数据真实性争议。您可以随便定义它，例如: `1111|2222|33333333`，不同段用 `|` 切开。
2. `./preconfigs/embeded/miaokoCA/miaoko.crt`: 不提供自行openssh弄
3. `./preconfigs/embeded/miaokoCA/miaoko.key`: 不提供
4. `./preconfigs/embeded/ca-certificates.crt`: miaospeed 自带的根证书集，防止有恶意用户修改系统更证书以作假 TLS RTT。（对于 debian 用户，您可以在安装 `ca-certificates` 包后，在 `/etc/ssl/certs/ca-certificates.crt` 获取这个文件）