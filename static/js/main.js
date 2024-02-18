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

function sendExpression() {
    // Получаем значение введенного выражения
    var expression = document.getElementById("expression").value;

    // Показываем пользователю, что выражение отправлено на обработку
    var statusElement = document.getElementById("status");
    statusElement.innerHTML = "Выражение: " + expression + ",<br> Статус: в обработке";

    // Создаем WebSocket соединение
    var socket = new WebSocket("ws://localhost:8080/ws");

    // Обработчик открытия соединения
    socket.onopen = function(event) {
        console.log("WebSocket connection opened");

        // Отправляем выражение на сервер
        socket.send(JSON.stringify({ expression: expression }));
    };

    var intervalId;

    // Обработчик получения сообщения от сервера
    socket.onmessage = function(event) {
        var data = JSON.parse(event.data);
        var agentId = data.id;
        console.log(1)
        if (data.result !== null && data.result !== undefined) {
            console.log(2)
            // Если результат получен, отображаем его
            statusElement.innerHTML = "Выражение: " + data.expression + ",<br> Результат: " + data.result;
            clearInterval(intervalId);
        } else {
            console.log(3)
            // Если результат не доступен, отправляем запрос на сервер для получения результата
            intervalId = setInterval(function() {
                console.log(4)
                socket.send(JSON.stringify({ agent_id: agentId }));
            }, 1000); // Повторяем запрос каждую секунду
        }
    };

    // Обработчик ошибок
    socket.onerror = function(error) {
        console.error('WebSocket Error:', error);
        statusElement.innerHTML = "Ошибка: " + error.message;
    };
}

function validateAndSend() {
    if (validateExpression()) {
        sendExpression();
    } else {
        console.log("нет");
    }
}