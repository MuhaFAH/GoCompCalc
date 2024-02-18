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

    // Проверка на операторы
    if (/(\+{2,}|-{2,}|\/{2,}|\*{2,})/.test(expression)) {
        alert("Выражение содержит недопустимое использование операторов.");
        return false;
    }

    // Проверка на последний символ
    var lastChar = expression.trim().slice(-1);
    if (lastChar === '+' || lastChar === '-' || lastChar === '*' || lastChar === '/' || lastChar === '.') {
        alert("Выражение не может заканчиваться оператором или точкой.");
        return false;
    }

    // Проверка на скобочность выражения
    if (/^[()]+$/.test(expression)) {
        alert("Выражение не может состоять только из скобок.");
        return false;
    }

    return true;
}

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
    return stack.length === 0; // Если всё ровно, то хорошо
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

    var socket = new WebSocket("ws://localhost:8080/ws");

    socket.onopen = function(event) {
        console.log("WebSocket соединение успешно установлено");
        socket.send(JSON.stringify({ expression: expression }));
    };

    var intervalId;

    socket.onmessage = function(event) {
        var data = JSON.parse(event.data);
        var agentId = data.id;
        if (data.result !== null && data.result !== undefined) {
            statusElement.innerHTML = data.expression;
            processText.innerHTML = "Результат";
            processResult.style.color = "green";
            processResult.innerHTML = data.result;
            clearInterval(intervalId);
        } else {
            intervalId = setInterval(function() {
                socket.send(JSON.stringify({ getresult: agentId }));
            }, 2000);
        }
    };

    socket.onerror = function(error) {
        console.error('WebSocket Error:', error);
        statusElement.innerHTML = data.expression;
        processText.innerHTML = "Результат";
        processResult.style.color = "red";
        processResult.innerHTML = "Ошибка";
        clearInterval(intervalId);
    };
}

function validateAndSend() {
    if (validateExpression()) {
        sendExpression();
    } else {
        console.log("нет");
    }
}