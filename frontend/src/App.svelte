<script>
  import { Greet } from './wailsjs/go/main/App.js';

  let name = '';
  let greeting = '';

  async function callGreet() {
    try {
      greeting = await Greet(name);
    } catch (e) {
      console.error(e);
      greeting = 'Erro ao se comunicar com o backend.';
    }
  }
</script>

<main>
  <h1>eprocIA</h1>
  <p>Bem-vindo ao eprocIA - ETL Judicial</p>
  
  <div class="test-bind">
    <input bind:value={name} placeholder="Digite seu nome..." />
    <button on:click={callGreet}>Enviar</button>
    {#if greeting}
      <p class="greeting">{greeting}</p>
    {/if}
  </div>
</main>

<style>
  main {
    text-align: center;
    padding: 2rem;
    max-width: 400px;
    margin: 0 auto;
  }
  
  .test-bind {
    margin-top: 2rem;
    display: flex;
    flex-direction: column;
    gap: 1rem;
    align-items: center;
  }

  input {
    padding: 0.5rem;
    border: 1px solid #DEE2E6;
    border-radius: 4px;
    width: 80%;
    font-size: 13px;
  }

  button {
    padding: 0.5rem 1rem;
    background-color: #0056b3;
    color: white;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    font-size: 13px;
    font-weight: 600;
  }

  button:hover {
    background-color: #004085;
  }

  .greeting {
    font-size: 13px;
    font-weight: bold;
    color: #0056b3;
  }
</style>
