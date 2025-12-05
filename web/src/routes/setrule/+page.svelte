<script lang="ts">
    import { onMount } from 'svelte';
    import { goto } from '$app/navigation';

    let apiKey = $state("");
    let isLoggedIn = $state(false);
    let availableMetrics = $state<string[]>([]);
    
    let ruleMetric = $state("");
    let ruleThreshold = $state(0);
    let ruleStatus = $state("");

    async function fetchMetrics() {
        if (!apiKey) return;

        try {
            const res = await fetch('/api/metrics', {
                headers: { 'X-API-Key': apiKey }
            });
            
            if (res.status === 401) {
                goto('/');
                return;
            }
            if (!res.ok) return;
            
            const data = await res.json();
            availableMetrics = [...new Set<string>(data.map((m: any) => m.name))].sort();

        } catch (err) {
            console.error(err);
        }
    }

    async function createRule() {
        if (!ruleMetric) {
            ruleStatus = "Select a metric first.";
            return;
        }
        
        ruleStatus = "Saving...";
        
        try {
            const res = await fetch('/api/rules', {
                method: 'POST',
                headers: { 
                    'X-API-Key': apiKey,
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    metric: ruleMetric,
                    threshold: Number(ruleThreshold)
                })
            });

            if (res.ok) {
                ruleStatus = "Rule Active!";
                ruleThreshold = 0;
            } else {
                ruleStatus = "Failed to save.";
            }
        } catch (e) {
            ruleStatus = "Network Error";
        }
    }

    function logout() {
        localStorage.removeItem("datacat_key");
        goto('/');
    }

    onMount(() => {
        const savedKey = localStorage.getItem("datacat_key");
        if (!savedKey) {
            goto('/');
            return;
        }
        apiKey = savedKey;
        isLoggedIn = true;
        fetchMetrics();
    });
</script>

<div style="background: #000; color: #fff; min-height: 100vh; padding: 20px; font-family: monospace;">
    
    <div style="margin-bottom: 20px;">
        <a href="/" style="color: #fff; text-decoration: none;">datacat</a>
        <a href="/setrule" style="margin-left: 20px; color: #fff;">[SET RULE]</a>
        <a href="/docs" style="margin-left: 10px; color: #fff;">[DOCS]</a>
        <a href="/about" style="margin-left: 10px; color: #fff;">[ABOUT]</a>
        <button onclick={logout} style="margin-left: 10px; background: none; border: none; color: #fff; cursor: pointer;">[LOGOUT]</button>
    </div>

    <hr style="border-color: #fff; margin: 20px 0;">

    <div>Create Alert Rule</div>
    <br>
    <div>Metric:</div>
    <select 
        bind:value={ruleMetric}
        style="background: #000; border: 1px solid #fff; color: #fff; padding: 5px; margin: 5px 0;"
    >
        <option value="">-- Select --</option>
        {#each availableMetrics as m (m)}
            <option value={m}>{m}</option>
        {/each}
    </select>
    <br>
    <div>Threshold:</div>
    <input 
        type="number" 
        bind:value={ruleThreshold}
        style="background: #000; border: 1px solid #fff; color: #fff; padding: 5px; margin: 5px 0; width: 100px;"
    />
    <br><br>
    <button 
        onclick={createRule}
        style="background: #fff; color: #000; border: none; padding: 5px 15px; cursor: pointer;"
    >
        SAVE RULE
    </button>
    {#if ruleStatus}
        <span style="margin-left: 10px;">{ruleStatus}</span>
    {/if}

</div>
