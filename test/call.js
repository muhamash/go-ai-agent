async function fetchOpenAICompletion(prompt) {
    const apiKey = ''; 

    const response = await fetch('https://api.openai.com/v1/chat/completions', {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${apiKey}`,
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({
        model: 'gpt-3.5-turbo',
        messages: [
          { role: 'system', content: 'You are a helpful assistant.' },
          { role: 'user', content: prompt }
        ],
        temperature: 0.7
      })
    });
  
    if (!response.ok) {
      const error = await response.json();
      throw new Error(`OpenAI API error: ${error.error.message}`);
    }
  
    const data = await response.json();
    return data.choices[0].message.content;
  }
  

fetchOpenAICompletion("What is the capital of France?")
  .then(reply => console.log("OpenAI says:", reply))
  .catch(error => console.error("Error:", error.message));
