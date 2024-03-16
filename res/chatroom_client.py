import socket
import threading
import tkinter as tk
from tkinter import messagebox
import ctypes

# 全局变量用于存储socket客户端对象和连接状态
client = None
connected = False
login_window = None

class LoginGUI:
    def __init__(self, master):
        self.master = master  # 存储传入的 Tk 实例
        self.master.title("Paizer客户端")
        self.master.geometry("280x150")  # 调整登录窗口大小
        # 设置窗口的最小和最大尺寸为当前大小，从而禁止调整窗口大小
        self.master.resizable(width=False, height=False)
        # 昵称标签和输入框
        self.nickname_label = tk.Label(self.master, text="键入您的昵称（支持中、英文和数字）:")
        self.nickname_label.pack()
        self.nickname_entry = tk.Entry(self.master, width=20)
        self.nickname_entry.pack()

        # 服务器IP地址标签和输入框
        self.server_ip_port_label = tk.Label(self.master, text="键入服务器IP地址(可选端口，默认21156):")
        self.server_ip_port_label.pack()
        self.server_ip_port_entry = tk.Entry(self.master, width=20)
        self.server_ip_port_entry.pack()

        # 登录按钮
        self.login_button = tk.Button(self.master, text="登录", command=self.login)
        self.login_button.pack(pady=10)

    def login(self):
        nickname = self.nickname_entry.get()
        server_ip_port = self.server_ip_port_entry.get()

        # 验证输入
        if validate_input(nickname, server_ip_port):
            # 如果验证通过，调用 on_connect 函数
            global login_window
            login_window = self.master  # 确保全局变量引用当前的登录窗口
            on_connect(nickname, server_ip_port, self.master)
            # 隐藏登录窗口
            self.master.withdraw()

class PaizerClientGUI:
    def __init__(self, master, nickname, server_ip_port):
        self.master = master
        self.master.title("Paizer客户端")
        self.master.geometry("570x470")  # 调整GUI窗口大小
        self.master.resizable(width=False, height=False)

        self.nickname = nickname
        self.server_ip_port = server_ip_port

        # 创建消息显示区域
        self.message_text = tk.Text(self.master, height=15, width=60, font=('Arial', 12))
        self.message_text.pack(expand=True, fill='both')

        # 创建滚动条
        self.scrollbar = tk.Scrollbar(self.master, command=self.message_text.yview)
        self.scrollbar.pack(side='right', fill='y')

        # 绑定滚动条和文本框
        self.message_text.config(yscrollcommand=self.scrollbar.set)

        # 创建底部框架，用于放置输入框和发送按钮
        bottom_frame = tk.Frame(self.master, bd=2, relief=tk.SUNKEN)
        bottom_frame.pack(side=tk.BOTTOM, fill=tk.X, padx=10, pady=10)

        # 输入框
        self.message_entry = tk.Entry(bottom_frame, width=50)
        self.message_entry.pack(side=tk.LEFT, fill=tk.X, padx=(10, 5), pady=(10, 10), expand=True)

        # 绑定回车键事件
        self.message_entry.bind('<Return>', self.send_message_on_enter)

        # 发送按钮
        self.send_button = tk.Button(bottom_frame, text="发送", command=self.send_message, width=15)
        self.send_button.pack(side=tk.LEFT, padx=(5, 10), pady=(10, 10))

        # 尝试连接到服务器
        self.create_connection()
        
        self.master.protocol("WM_DELETE_WINDOW", self.on_close)

    def on_close(self):
        # 询问用户是否确实想要退出
        choice = messagebox.askyesno("退出确认", "您确定要退出Paizer客户端吗？")
        if choice:  # 用户点击“是”
            # 尝试安全地关闭socket连接
            if client is not None:
                client.close()
            # 强制结束程序
            ctypes.windll.kernel32.ExitProcess(0)
        # 用户点击“否”，不做任何操作，窗口保持打开状态

    # 新的方法，当按下回车键时调用
    def send_message_on_enter(self, event):
        self.send_message()

    def create_connection(self):
        global client, connected
        if not connected:
            try:
                # 解析服务器IP和端口
                if ':' in self.server_ip_port:
                    server_ip, port_str = self.server_ip_port.split(':')
                    port = int(port_str)
                else:
                    server_ip = self.server_ip_port
                    port = 21156  # 默认端口号

                # 创建socket并连接到服务器
                client = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
                client.connect((server_ip, port))
                client.send(f"{self.nickname}".encode('utf-8'))
                connected = True
                self.receive_thread = threading.Thread(target=self.receive_message)
                self.receive_thread.start()
                self.log_message("Paizer客户端")
                self.log_message(f"成功登录到服务器 {server_ip}:{port}\n")
            except Exception as e:
                self.log_message(f"[!] 无法连接至服务器 {self.server_ip_port}: {e}")
                if client is not None:
                    client.close()
                connected = False

    def send_message(self):
        message = self.message_entry.get()
        if message:
            # 格式化消息为 "用户名: 消息内容"
            formatted_message = f"{self.nickname}: {message}"
            client.send(formatted_message.encode('utf-8'))
            self.message_entry.delete(0, tk.END)

    def receive_message(self):
        while True:
            try:
                message = client.recv(1024).decode('utf-8')
                # 假设消息格式为 "昵称: 消息内容"，且不包含发送者的IP地址
                self.log_message(message)
            except Exception as e:
                self.log_message(f"[!] 接收消息时发生错误: {e}")
                if client is not None:
                    client.close()
                connected = False
                break

    def log_message(self, message):
        # 将消息添加到文本框中
        self.message_text.config(state='normal')  # 允许编辑文本框
        self.message_text.insert(tk.END, message + '\n')
        self.message_text.config(state='disabled')  # 禁止编辑文本框
        self.message_text.yview(tk.END)  # 自动滚动到最新消息

def validate_input(nickname, server_ip_port, parent=None):
    if not nickname.strip():
        messagebox.showwarning("警告！", "昵称不能为空。", parent=parent)
        return False
    # 检查是否包含端口号
    try:
        # 如果包含冒号，则分割IP和端口
        if ':' in server_ip_port:
            server_ip, port_str = server_ip_port.split(':')
            port = int(port_str)
        else:
            # 否则，使用默认端口号
            server_ip = server_ip_port
            port = 21156
    except ValueError:
        messagebox.showwarning("警告！", "请输入有效的服务器IP地址和端口号。", parent=parent)
        return False
    # 检查IP地址是否有效
    if not server_ip.strip():
        messagebox.showwarning("警告！", "请输入有效的服务器IP地址。", parent=parent)
        return False
    return True

def on_connect(nickname, server_ip_port, login_root):
    # 隐藏登录窗口
    login_root.withdraw()

    # 创建新的 GUI 窗口实例
    root = tk.Tk()
    root.title("Paizer客户端")
    app = PaizerClientGUI(root, nickname, server_ip_port)
    
    # 启动新的 GUI 窗口的主循环
    root.mainloop()


def main():
    # ctypes.windll.kernel32.FreeConsole()
    # 创建登录窗口
    login_root = tk.Tk()
    global login_window
    login_window = login_root
    login_app = LoginGUI(login_root)
    login_root.mainloop()

if __name__ == "__main__":
    main()