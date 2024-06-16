#使用golang镜像的版本
FROM golang:1.19-alpine3.16

# 环境变量,linux系统下的环境配置
# 注意要设置或内代理，否则可能编译不过
ENV GO111MODULE=on \
    GOPROXY='https://proxy.golang.com.cn,direct' \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64
ENV TZ=Asia/Shanghai
# RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

# 移动到工作目录：/workdir
WORKDIR /app

# 将代码复制到容器中
COPY . .

WORKDIR /app/src
# 编译成二进制可执行文件 main
RUN go build -ldflags="-s -w" -o main main.go
ENTRYPOINT ["/app/src/main"]