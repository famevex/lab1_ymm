// Тестовая программа на C++

#include <iostream>

/* Функция сложения двух чисел */
int add(int a, int b) {
    return a + b; // возвращаем сумму
}

int main() {
    /* Объявление переменных */
    int x = 10;
    int y = 3;
    int result = 0;

    // Арифметические выражения
    int product = x * y;
    int diff = x - y;

    // Логическое выражение
    bool isPositive = (x > 0) && (y > 0);

    // Условный оператор if-else
    if (isPositive) {
        std::cout << "Оба положительные" << std::endl;
    } else {
        std::cout << "Не положительные" << std::endl;
    }

    // Цикл for
    for (int i = 1; i <= 5; i++) {
        std::cout << "i = " << i << std::endl;
    }

    // Цикл while
    int count = 0;
    while (count < 3) {
        count++;
    }

    // Вызов функции
    result = add(x, y);
    std::cout << "Сумма: " << result << std::endl;

    
    /* Строки со спецсимволами // внутри не должны чиститься */
    std::string s = "Hello // world /* test */";

    return 0;
}