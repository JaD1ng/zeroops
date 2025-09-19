MockS3 部署全流程总结

1. 文件传输到服务器

# 在服务器上执行
cd /tmp/dingnanjia

2. 解压部署包

# 解压
tar -xzf mock-s3-*.tar.gz

# 验证内容
ls -la
# 应该看到: bin/ config/ scripts/ 目录

3. 执行部署

chmod +x deploy.sh
./deploy.sh

这个脚本会：
- 创建目录结构 /home/qboxserver/zeroops_*
- 部署二进制文件到 _package 目录
- 部署配置文件到 _package/config 目录
- 创建启动脚本 manual-start.sh 和 manual-stop.sh

4. 部署基础设施（Docker）

# 启动 PostgreSQL、Redis、Consul 等
docker-compose -f docker-compose.infra.yml up -d

# 验证
docker-compose -f docker-compose.infra.yml ps

5. 启动业务服务

方式A：手动启动（推荐，无需sudo）

./manual-start.sh

方式B：使用 Supervisor（需要sudo）

# 安装配置
sudo cp mock-s3-simple.conf /etc/supervisord/zeroops-mock-s3.conf

# 启动服务
sudo supervisorctl reread
sudo supervisorctl update
sudo supervisorctl start zeroops_*

6. 验证服务

# 查看进程
ps aux | grep zeroops_

# 健康检查
```
for port in 8182 8183 8191 8192 8201 8202 8211 8221; do
    echo "Checking port $port:"
    curl -s -o /dev/null -w "HTTP Status: %{http_code}\n" http://localhost:$port/metrics ||echo "Failed"
done
```

# 查看日志
tail -f /home/qboxserver/zeroops_metadata_1/logs/service.log

# Supervisor 状态（如使用）
supervisorctl status | grep zeroops_

7. 服务管理

# 停止服务
./manual-stop.sh
# 或
supervisorctl stop zeroops_*

# 重启服务
./manual-stop.sh && ./manual-start.sh
# 或
supervisorctl restart zeroops_*

# 查看特定服务日志
supervisorctl tail -f zeroops_metadata_1

目录结构

/home/qboxserver/
├── zeroops_metadata_1/
│   ├── _package/
│   │   ├── metadata-service     # 二进制文件
│   │   ├── config/
│   │   │   └── metadata-config.yaml
│   │   └── start.sh
│   ├── logs/
│   └── data/
├── zeroops_storage_1/
│   └── ... (类似结构)
└── ... (其他服务)

端口分配

| 服务          | 实例  | 端口        |
|-------------|-----|-----------|
| metadata    | 1-3 | 8181-8183 |
| storage     | 1-2 | 8191-8192 |
| queue       | 1-2 | 8201-8202 |
| third_party | 1   | 8211      |
| mock_error  | 1   | 8221      |

故障排查

# 如果服务无法启动，检查配置文件
cat /home/qboxserver/zeroops_queue_1/_package/config/queue-config.yaml

# 检查端口占用
netstat -tlnp | grep 81

# 查看错误日志
tail -100 /home/qboxserver/zeroops_*/logs/*.log

# 手动测试启动
cd /home/qboxserver/zeroops_metadata_1/_package
./metadata-service --port=8181