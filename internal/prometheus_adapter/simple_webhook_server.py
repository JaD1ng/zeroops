#!/usr/bin/env python3
"""
简单的 Webhook 服务器 - 使用标准库接收 Alertmanager 告警
"""

from http.server import HTTPServer, BaseHTTPRequestHandler
import json
from datetime import datetime
import threading
import time

# 存储接收到的告警
alerts_received = []
alerts_lock = threading.Lock()

class WebhookHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        """处理 POST 请求"""
        if self.path == '/v1/integrations/alertmanager/webhook':
            try:
                # 读取请求体
                content_length = int(self.headers['Content-Length'])
                post_data = self.rfile.read(content_length)

                # 解析 JSON
                data = json.loads(post_data.decode('utf-8'))

                # 记录告警
                timestamp = datetime.now().isoformat()
                with alerts_lock:
                    alert_record = {
                        "timestamp": timestamp,
                        "data": data
                    }
                    alerts_received.append(alert_record)

                    # 只保留最近100条
                    if len(alerts_received) > 100:
                        alerts_received.pop(0)

                # 打印到控制台
                print(f"\n[{timestamp}] 收到告警:")
                print(json.dumps(data, indent=2, ensure_ascii=False))

                # 提取并显示关键信息
                if 'alerts' in data:
                    print("\n告警摘要:")
                    for alert in data['alerts']:
                        alert_name = alert.get('labels', {}).get('alertname', 'Unknown')
                        status = alert.get('status', 'Unknown')
                        severity = alert.get('labels', {}).get('severity', 'Unknown')
                        service = alert.get('labels', {}).get('service', 'N/A')
                        print(f"  - {alert_name}: {status} (severity: {severity}, service: {service})")

                print("-" * 50)

                # 返回成功响应
                self.send_response(200)
                self.send_header('Content-Type', 'application/json')
                self.end_headers()
                response = {"status": "success", "message": "Alert received"}
                self.wfile.write(json.dumps(response).encode())

            except Exception as e:
                print(f"Error processing alert: {e}")
                self.send_response(400)
                self.send_header('Content-Type', 'application/json')
                self.end_headers()
                response = {"status": "error", "message": str(e)}
                self.wfile.write(json.dumps(response).encode())
        else:
            self.send_response(404)
            self.end_headers()
            self.wfile.write(b"Not Found")

    def do_GET(self):
        """处理 GET 请求"""
        if self.path == '/alerts':
            # 返回接收到的告警列表
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.end_headers()
            with alerts_lock:
                self.wfile.write(json.dumps(alerts_received, indent=2).encode())

        elif self.path == '/health':
            # 健康检查
            self.send_response(200)
            self.end_headers()
            self.wfile.write(b"OK")

        elif self.path == '/':
            # 首页
            self.send_response(200)
            self.send_header('Content-Type', 'text/html')
            self.end_headers()
            html = """
            <html>
            <head><title>Webhook Server</title></head>
            <body>
                <h1>Mock Webhook Server</h1>
                <p>Webhook endpoint: POST /v1/integrations/alertmanager/webhook</p>
                <p>View alerts: <a href="/alerts">GET /alerts</a></p>
                <p>Health check: <a href="/health">GET /health</a></p>
                <hr>
                <p>Alerts received: {}</p>
            </body>
            </html>
            """.format(len(alerts_received))
            self.wfile.write(html.encode())
        else:
            self.send_response(404)
            self.end_headers()
            self.wfile.write(b"Not Found")

    def log_message(self, format, *args):
        """自定义日志格式"""
        return  # 禁用默认的日志输出，避免太多噪音

def run_server(port=8080):
    """运行服务器"""
    server_address = ('', port)
    httpd = HTTPServer(server_address, WebhookHandler)

    print("=" * 60)
    print("Mock Webhook Server 已启动")
    print(f"监听地址: http://0.0.0.0:{port}")
    print(f"Webhook 端点: POST /v1/integrations/alertmanager/webhook")
    print(f"查看告警: GET /alerts")
    print(f"健康检查: GET /health")
    print("=" * 60)
    print("\n等待接收告警...\n")

    try:
        httpd.serve_forever()
    except KeyboardInterrupt:
        print("\n服务器已停止")
        httpd.shutdown()

if __name__ == '__main__':
    import sys

    # 检查是否指定端口
    port = 8080
    if len(sys.argv) > 1:
        try:
            port = int(sys.argv[1])
        except ValueError:
            print(f"无效的端口: {sys.argv[1]}")
            sys.exit(1)

    # 检查端口是否被占用
    import socket
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    result = sock.connect_ex(('127.0.0.1', port))
    sock.close()

    if result == 0:
        print(f"警告: 端口 {port} 已被占用")
        print("你可以:")
        print(f"1. 使用其他端口: python3 {sys.argv[0]} 8081")
        print(f"2. 或者停止占用端口 {port} 的服务")
        response = input(f"\n是否继续在端口 {port} 上启动? (y/N): ")
        if response.lower() != 'y':
            sys.exit(0)

    run_server(port)