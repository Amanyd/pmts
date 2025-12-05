<script lang="ts">
    import { onMount } from 'svelte';

    let isSignupMode = $state(false);
    let email = $state("");
    let generatedKey = $state("");
    
    let apiKey = $state("");
    let isLoggedIn = $state(false);
    let errorMsg = $state("");
    let metrics = $state<any[]>([]); 
    let availableMetrics = $state<string[]>([]);
    let selectedMetric = $state("");
    let currentValue = $state(0);
    let isConnected = $state(false);

    async function register() {
        if (!email.includes("@")) {
            errorMsg = "Please enter a valid email address.";
            return;
        }

        errorMsg = "";
        generatedKey = "Processing...";

        try {
            const res = await fetch('/api/register', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ email: email })
            });

            const data = await res.json();
            
            if (res.status !== 200) {
                generatedKey = "";
                errorMsg = data.error || "Signup failed (Email may exist).";
                return;
            }

            generatedKey = data.api_key;
            errorMsg = "";

        } catch (e) {
            generatedKey = "";
            errorMsg = "Network error. Is the backend running?";
        }
    }

    function login() {
        if (!apiKey.startsWith("sk_")) {
            errorMsg = "Invalid Key Format (must start with sk_)";
            return;
        }
        localStorage.setItem("datacat_key", apiKey);
        isLoggedIn = true;
        fetchMetrics();
    }

    function logout() {
        localStorage.removeItem("datacat_key");
        isLoggedIn = false;
        apiKey = "";
        generatedKey = "";
    }

    async function fetchMetrics() {
        if (!isLoggedIn) return;

        try {
            const res = await fetch('/api/metrics', {
                headers: { 'X-API-Key': apiKey }
            });
            
            if (res.status === 401) {
                errorMsg = "Invalid API Key";
                isLoggedIn = false;
                return;
            }
            if (!res.ok) throw new Error("API Error");
            
            const data = await res.json();
            isConnected = true;
            errorMsg = "";

            const newMetrics = [...new Set<string>(data.map((m: any) => m.name))].sort();
            const currentSelection = selectedMetric;
            
            if (JSON.stringify(newMetrics) !== JSON.stringify(availableMetrics)) {
                availableMetrics = newMetrics;
            }
            if (!currentSelection && availableMetrics.length > 0) {
                selectedMetric = availableMetrics[0];
            } else if (currentSelection && availableMetrics.includes(currentSelection)) {
                selectedMetric = currentSelection;
            }
            const selectedData = data.find((m: any) => m.name === selectedMetric);
            if (selectedData && selectedData.samples.length > 0) {
                const lastSample = selectedData.samples[selectedData.samples.length - 1];
                currentValue = lastSample.v;
                metrics = selectedData.samples;
            }

        } catch (err) {
            isConnected = false;
        }
    }

    onMount(() => {
        const savedKey = localStorage.getItem("datacat_key");
        if (savedKey) {
            apiKey = savedKey;
            isLoggedIn = true;
            fetchMetrics();
        }

        const interval = setInterval(fetchMetrics, 2000);
        return () => clearInterval(interval);
    });
</script>

<div style="background: #000; color: #fff; min-height: 100vh; padding: 20px; font-family: monospace;">
    
    <div style="margin-bottom: 20px;">
        <a href="/" style="color: #fff; text-decoration: none;">datacat</a>
        {#if isLoggedIn}
            <a href="/setrule" style="margin-left: 20px; color: #fff;">[SET RULE]</a>
            <a href="/docs" style="margin-left: 10px; color: #fff;">[DOCS]</a>
            <a href="/about" style="margin-left: 10px; color: #fff;">[ABOUT]</a>
            <button onclick={logout} style="margin-left: 10px; background: none; border: none; color: #fff; cursor: pointer;">[LOGOUT]</button>
        {/if}
    </div>

    <div>Status: {isConnected ? 'CONNECTED' : 'DISCONNECTED'}</div>

    <hr style="border-color: #fff; margin: 20px 0;">

    {#if !isLoggedIn}
        <div>
            
            {#if generatedKey}
                <div>SUCCESS! Your new API Key:</div>
                <div style="margin: 10px 0;">{generatedKey}</div>
                <div>^ SAVE THIS KEY! It is your password.</div>
                <br>
                <button 
                    onclick={() => {isSignupMode = false; apiKey = generatedKey; login();}}
                    style="background: #fff; color: #000; border: none; padding: 5px 15px; cursor: pointer;"
                >
                    Log In Now
                </button>

            {:else}
                <div>{isSignupMode ? 'Get New API Key' : 'Login with API Key'}</div>
                <br>

                {#if isSignupMode}
                    <div>Email:</div>
                    <input 
                        type="email" 
                        bind:value={email} 
                        placeholder="email@example.com" 
                        style="background: #000; border: 1px solid #fff; color: #fff; padding: 5px; margin: 10px 0; width: 300px;"
                    />
                    <br>
                    <button 
                        onclick={register}
                        style="background: #fff; color: #000; border: none; padding: 5px 15px; cursor: pointer;"
                    >
                        CREATE NEW KEY
                    </button>
                    <br><br>
                    <button 
                        onclick={() => isSignupMode = false}
                        style="background: none; border: none; color: #fff; cursor: pointer;"
                    >
                        [I already have a key]
                    </button>
                {:else}
                    <div>API Key:</div>
                    <input 
                        type="text" 
                        bind:value={apiKey} 
                        placeholder="sk_..." 
                        style="background: #000; border: 1px solid #fff; color: #fff; padding: 5px; margin: 10px 0; width: 300px;"
                    />
                    <br>
                    <button 
                        onclick={login}
                        style="background: #fff; color: #000; border: none; padding: 5px 15px; cursor: pointer;"
                    >
                        LOGIN
                    </button>
                    <br><br>
                    <button 
                        onclick={() => isSignupMode = true}
                        style="background: none; border: none; color: #fff; cursor: pointer;"
                    >
                        [Need an API Key? Sign Up]
                    </button>
                {/if}

                {#if errorMsg}
                    <div style="margin-top: 10px;">Error: {errorMsg}</div>
                {/if}
            {/if}
        </div>

    {:else}
        <div>
            <div>Select Metric:</div>
            <select 
                bind:value={selectedMetric}
                style="background: #000; border: 1px solid #fff; color: #fff; padding: 5px; margin: 10px 0;"
            >
                {#if availableMetrics.length === 0}
                    <option>Waiting for data...</option>
                {/if}
                {#each availableMetrics as m (m)}
                    <option value={m}>{m}</option>
                {/each}
            </select>

            <div style="margin: 20px 0;">
                <div>Current Value:</div>
                <div style="font-size: 32px;">{currentValue.toFixed(4)}</div>
            </div>

            <hr style="border-color: #fff; margin: 20px 0;">

            <div>Raw Samples (last 20):</div>
            <pre style="margin-top: 10px;">{#each metrics.slice(-20) as sample}
t: {sample.t} | v: {sample.v}
{/each}</pre>

            {#if metrics.length === 0}
                <div>No data. Run the agent.</div>
            {/if}
        </div>
    {/if}

</div>