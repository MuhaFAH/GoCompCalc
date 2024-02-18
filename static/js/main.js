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

    // Проверка на последний символ оператора
    var lastChar = expression.trim().slice(-1);
    if (lastChar === '+' || lastChar === '-' || lastChar === '*' || lastChar === '/' || lastChar === '.') {
        alert("Выражение не может заканчиваться оператором или точкой.");
        return false;
    }

    if (/^[()]+$/.test(expression)) {
        alert("Выражение не может состоять только из скобок.");
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
    var expression = document.getElementById("expression").value;

    var statusElement = document.getElementById("status");
    var statusText = document.getElementById("status_text");
    var processText = document.getElementById("process_text");
    var processResult = document.getElementById("process_result");
    statusText.style.display = 'block';
    processText.innerHTML = 'Статус';
    processText.style.display = 'block';
    processResult.innerHTML = 'В обработке';
    processResult.style.color = "orange";
    processResult.style.display = 'block';
    statusElement.innerHTML = expression;
    statusElement.style.display = 'block';

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
            statusElement.innerHTML = data.expression;
            processText.innerHTML = "Результат";
            processResult.style.color = "green";
            processResult.innerHTML = data.result;
            clearInterval(intervalId);
        } else {
            console.log(3)
            // Если результат не доступен, отправляем запрос на сервер для получения результата
            intervalId = setInterval(function() {
                console.log(4)
                socket.send(JSON.stringify({ agent_id: agentId }));
            }, 2000); // Повторяем запрос каждую секунду
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