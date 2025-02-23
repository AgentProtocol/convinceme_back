function displayMessage(data) {
    const messageContainer = document.createElement('div');
    messageContainer.className = `message ${data.agent ? 'agent-message' : 'player-message'}`;

    // Create header with agent name and conviction score
    if (data.agent && data.scores && data.scores.conviction) {
        const messageHeader = document.createElement('div');
        messageHeader.className = 'message-header';
        
        const convictionBar = document.createElement('div');
        convictionBar.className = 'conviction-bar';
        convictionBar.innerHTML = `
            <div class="conviction-label">Conviction:</div>
            <div class="conviction-meter">
                <div class="conviction-fill" style="width: ${data.scores.conviction.score}%"></div>
                <span class="conviction-score">${data.scores.conviction.score}%</span>
            </div>
        `;
        
        messageHeader.innerHTML = `<span class="agent-name">${data.agent}</span>`;
        messageHeader.appendChild(convictionBar);
        messageContainer.appendChild(messageHeader);
    }

    // Create message content
    const messageContent = document.createElement('div');
    messageContent.className = 'message-content';
    messageContent.textContent = data.message || '';
    messageContainer.appendChild(messageContent);

    // Add scores if present
    if (data.scores && data.scores.argument) {
        const scoresContainer = document.createElement('div');
        scoresContainer.className = 'message-scores';
        
        const score = data.scores.argument;
        scoresContainer.innerHTML = `
            <div class="score-header">Argument Analysis</div>
            <div class="score-grid">
                <div class="score-item">
                    <div class="score-label">Overall</div>
                    <div class="score-bar">
                        <div class="score-fill" style="width: ${score.average}%"></div>
                        <span class="score-value">${score.average.toFixed(1)}</span>
                    </div>
                </div>
                <div class="score-item">
                    <div class="score-label">Strength</div>
                    <div class="score-bar">
                        <div class="score-fill" style="width: ${score.strength}%"></div>
                        <span class="score-value">${score.strength}</span>
                    </div>
                </div>
                <div class="score-item">
                    <div class="score-label">Relevance</div>
                    <div class="score-bar">
                        <div class="score-fill" style="width: ${score.relevance}%"></div>
                        <span class="score-value">${score.relevance}</span>
                    </div>
                </div>
                <div class="score-item">
                    <div class="score-label">Logic</div>
                    <div class="score-bar">
                        <div class="score-fill" style="width: ${score.logic}%"></div>
                        <span class="score-value">${score.logic}</span>
                    </div>
                </div>
                <div class="score-item">
                    <div class="score-label">Truth</div>
                    <div class="score-bar">
                        <div class="score-fill" style="width: ${score.truth}%"></div>
                        <span class="score-value">${score.truth}</span>
                    </div>
                </div>
                <div class="score-item">
                    <div class="score-label">Humor</div>
                    <div class="score-bar">
                        <div class="score-fill" style="width: ${score.humor}%"></div>
                        <span class="score-value">${score.humor}</span>
                    </div>
                </div>
            </div>
            <div class="score-explanation">${score.explanation}</div>
        `;
        messageContainer.appendChild(scoresContainer);
    }

    messagesContainer.appendChild(messageContainer);
    messagesContainer.scrollTop = messagesContainer.scrollHeight;
} 