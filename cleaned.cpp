#include <iostream>
int add(int a, int b) {
return a + b;
}
int main() {
int x = 10;
int y = 3;
int result = 0;
int product = x * y;
int diff = x - y;
bool isPositive = (x > 0) && (y > 0);
if (isPositive) {
std::cout << "Оба положительные" << std::endl;
} else {
std::cout << "Не положительные" << std::endl;
}
for (int i = 1; i <= 5; i++) {
std::cout << "i = " << i << std::endl;
}
int count = 0;
while (count < 3) {
count++;
}
result = add(x, y);
std::cout << "Сумма: " << result << std::endl;
std::string s = "Hello // world /* test */";
return 0;
}
