# tgfreesub
一款自动从tg公开频道中抓取免费订阅的程序；自带web页面

# 截图
![web页面截图](https://github.com/nasbump/tgfreesub/blob/main/screenshots/web1.png)
![手机页面截图](https://github.com/nasbump/tgfreesub/blob/main/screenshots/web2.png)

# 编译安装 
项目纯go实现，直接拉代码编译：
```
git clone https://github.com/nasbump/tgfreesub.git
cd tgfreesub
./make.sh
```

# 启动
```
$ ./tgfreesub
usage: ./tgfreesub options
  -appid   ## 从tg官方申请
  -apphash ## https://core.telegram.org/api/obtaining_api_id
  -phone   ## 手机号
  -history 0  ## 每次启动时获取历史消息的条数
  -server 127.0.0.1:2010  ## http server listen addr
  -names   ## 频道名，可以有多个,如：schpd,fq521,xhjvpn,fq5211,fqzw9
  -session ./session.json  ## session file
  -redis redis://127.0.0.1:6379/0  ## 数据保存在Redis中
```

## 注意
- 首次启动时，需要登陆，并需要输入验证码；成功之后可以不用再登陆
- 频道名，从TG中获取链接，如：t.me/fqzw9，则取fqzw9为频道名

