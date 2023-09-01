## GO远程构建并调试

之前写C++，一直习惯了本地IDE+Remote CMake/GDB编译调试的模式。

因为6.824课程需要用GO，好像没有特别好的支持。记录一下如何配置调试的。

---

> IDE: Goland
>
> 操作系统：Windows
>
> 远程服务器：Ubuntu

1. 首先配置SSH,让其可以连接到服务器

![image-20230901093829409](https://antio2-1258695065.cos.ap-chengdu.myqcloud.com/img/blogimage-20230901093829409.png)

2. 配置部署。选择SFTP。在映射中选择上传的路径。

![image-20230901093937968](https://antio2-1258695065.cos.ap-chengdu.myqcloud.com/img/blogimage-20230901093937968.png)

这样就实现了本地和服务器文件的同步

3. 在服务器上安装delve

因为是ubuntu,我是直接`sudo apt install delve`就能进行安装。

> 但是后面发现直接这样安装的话版本有冲突

然后使用`dlv version`进行安装检查



使用源码进行安装：

```bash
cd ~

git clone git@github.com:go-delve/delve.git

cd delve

go install github.com/go-delve/delve/cmd/dlv
```



这个时候在你的go目录下，（比如我的是`~/go/bin`）会出现名字叫`dlv`的可执行文件。



然后将该路径添加到环境变量就行了。

![image-20230901161358207](https://antio2-1258695065.cos.ap-chengdu.myqcloud.com/img/blogimage-20230901161358207.png)

此时`dlv version`可以正确显示版本

```
 dlv version
Delve Debugger
Version: 1.21.0
Build: $Id: fec0d226b2c2cce1567d5f59169660cf61dc1efe $
```



4. 编写测试文件

```GO
package main

import (
    "fmt"
    "runtime"
)

func main() {
    fmt.Println("Hello Go")
    showOS()
}

func showOS() {
    os := runtime.GOOS
    fmt.Println("当前操作系统是:", os)
}
```

![image-20230901094548982](https://antio2-1258695065.cos.ap-chengdu.myqcloud.com/img/blogimage-20230901094548982.png)

测试代码说明：创建目录`test`,并且创建`go.mod`文件。

---

5. 配置Go Remote

![image-20230901094742377](https://antio2-1258695065.cos.ap-chengdu.myqcloud.com/img/blogimage-20230901094742377.png)

然后使用终端进入你的服务器代码路径，比如我的是`~/projects/6.824/test/hello`。

按照提示运行

`dlv debug --headless --listen=:2345 --api-version=2 --accept-multiclient`

6. 进行运行配置

这里运行于选择之前部署的服务器。然后在远程目标上构建

![image-20230901162914900](https://antio2-1258695065.cos.ap-chengdu.myqcloud.com/img/blogimage-20230901162914900.png)

7. 进行调试

可以看到，此时已经可以在服务器上构建并单步调试代码了![image-20230901163123113](https://antio2-1258695065.cos.ap-chengdu.myqcloud.com/img/blogimage-20230901163123113.png)