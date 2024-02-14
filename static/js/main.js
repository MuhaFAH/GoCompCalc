function validateExpression() {
    var expression = document.getElementById("expression").value;

    // Проверка на пустое значение
    if (!expression) {
        alert("Пожалуйста, введите выражение.");
        return false;
    }

    // Проверка на допустимые символы
    var validChars = /^[0-9+\-*/().\s]+$/;
    if (!validChars.test(expression)) {
        alert("Выражение содержит недопустимые символы.");
        return false;
    }

    // Проверка на баланс скобок
    if (!isBalanced(expression)) {
        alert("Выражение содержит непарные скобки.");
        return false;
    }

    if (/(\+{2,}|-{2,}|\/{2,}|\*{2,})/.test(expression)) {
        alert("Выражение содержит недопустимое использование операторов.");
        return false;
    }

    return true;
}

// Функция для проверки баланса скобок
function isBalanced(expression) {
    var stack = [];
    for (var i = 0; i < expression.length; i++) {
        var char = expression.charAt(i);
        if (char === '(') {
            stack.push(char);
        } else if (char === ')') {
            if (stack.length === 0) {
                return false; // Непарная закрывающая скобка
            }
            stack.pop();
        }
    }
    return stack.length === 0; // Должно быть пустое после обхода
}