# go_BreakpointdownloadwithProxy
一个断点下载的程序，包括server端、代理端和客户端

分别将server.go编译为server:go build server.go
pull.go编译为pull:go build pull.go
proxy.c编译为proxy:gcc -o proxy proxy.c


先启动server proxy ，然后在能够启动的所有代理机器上运行proxy ，最后启动pull proxy 。可执行文件在Linux环境下运行，程序通过测试选择一个外网速度最快的代理(测试不需要消耗额外的流量)来下载文件。

各个模块分别的启动方法如下：

pull 
功能：
拉取文件的客户端

参数：
./ pull  -h
Usage of ./downloader：
  -csize=8192: cache大小的设置
  -d="10.12.212.122:7777": 服务器地址：ip:port
  -n=0: 代理的数目
  -p="172.16.9.141:8050,172.16.9.141:8090,172.16.9.141:8070": 代理地址:port:id[,port:id...]
  -r="data_phase_two.dat": 请求的文件位置
  -s="/data/tegcode/data/data_phase_two.dat": 下载文件的存储位置
  -t=150: 最大连接数

可以调整最大的连接数，cache的大小，以及代理的数目和地址，服务器的地址，请求下载的文件，以及下载的文件存入的路径（如果存在同名的文件，则这个文件名后面加上.1）



用法：
-r和-s，-t,以及-csize的参数使用默认值就好，特殊情况需要指定
1.不使用代理的情况：./pull   –d ip:port
2.使用proxy的情况：./pull  –d ip:port  –n 3 –p port:id[,port:id...]

所以在提供代理的情况下，请使用如下命令来启动客户端下载程序：
./pull  –d ip:port  –n 3 –p port:id[,port:id...]

当然，这里pull一共给出了三种用法，建议在不同的情况下选择不同的代理。另外，后续将补充一种动态选择代理的算法，从而达到最佳下载速度。。

Server 
功能：提供http文件服务器的功能

参数：
./server  -h
Usage of ./server :
  -f="/data/tegcode/team58/": 文件服务器的文件路径
  -p="7777": 服务器的端口号

用法：
使用默认参数能够满足题目需求
./server  –p port –f /data/tegcode/team58/


Proxy 
功能： 
在客户端和服务器之间转发数据

参数：
./proxy  -l local_port -d remote_host -p remote_port [-i "input parser"] [-o "output parser"]
-l local_port:代理的本地端口号
-d remote_host：服务器的地址
-p remote_port：远程服务器的端口

用法：
根据题目要求，需要指定端口和服务器的IP和端口号，如下
./proxy  -l local_port -d remote_host -p remote_port
