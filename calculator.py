#!/usr/bin/env python3
"""
简单计算器 - 支持加减乘除运算
"""

def add(a, b):
    """加法"""
    return a + b

def subtract(a, b):
    """减法"""
    return a - b

def multiply(a, b):
    """乘法"""
    return a * b

def divide(a, b):
    """除法"""
    if b == 0:
        raise ValueError("除数不能为零！")
    return a / b

def get_number(prompt):
    """获取用户输入的数字"""
    while True:
        try:
            return float(input(prompt))
        except ValueError:
            print("无效输入，请输入一个数字！")

def get_operation():
    """获取用户选择的运算"""
    print("\n请选择运算：")
    print("1. 加法 (+)")
    print("2. 减法 (-)")
    print("3. 乘法 (*)")
    print("4. 除法 (/)")
    print("5. 退出 (q)")

    while True:
        choice = input("请输入选项 (1-5): ").strip().lower()

        if choice in ['1', '2', '3', '4']:
            return choice
        elif choice in ['5', 'q']:
            return 'quit'
        else:
            print("无效选项，请重新输入！")

def calculate():
    """主计算函数"""
    print("=" * 40)
    print("欢迎使用简单计算器！")
    print("=" * 40)

    while True:
        operation = get_operation()

        if operation == 'quit':
            print("\n感谢使用，再见！")
            break

        # 获取两个操作数
        print("\n请输入两个数字：")
        num1 = get_number("第一个数字: ")
        num2 = get_number("第二个数字: ")

        # 执行运算
        try:
            if operation == '1':
                result = add(num1, num2)
                symbol = '+'
            elif operation == '2':
                result = subtract(num1, num2)
                symbol = '-'
            elif operation == '3':
                result = multiply(num1, num2)
                symbol = '*'
            else:  # operation == '4'
                result = divide(num1, num2)
                symbol = '/'

            print(f"\n结果: {num1} {symbol} {num2} = {result}")

        except ValueError as e:
            print(f"\n错误: {e}")

        # 询问是否继续
        while True:
            continue_choice = input("\n是否继续计算？(y/n): ").strip().lower()
            if continue_choice in ['y', 'yes']:
                break
            elif continue_choice in ['n', 'no']:
                print("\n感谢使用，再见！")
                return
            else:
                print("无效输入，请输入 y 或 n！")

if __name__ == "__main__":
    calculate()
