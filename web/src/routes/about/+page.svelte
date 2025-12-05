<script lang="ts">
    import { onMount } from 'svelte';
    import { goto } from '$app/navigation';

    let apiKey = $state("");

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

    <div style="font-size: 18px;">About DataCat</div>
    <br>
    <div>DataCat is a lightweight metrics monitoring platform for developers.</div>
    <br>
    <div>Features:</div>
    <div>- Real-time system metrics (CPU, Memory, Disk, Network)</div>
    <div>- Custom application metrics via scraping</div>
    <div>- Serverless SDK for Node.js applications</div>
    <div>- Alert rules with threshold monitoring</div>
    <div>- Simple API key authentication</div>
    <br>
    <hr style="border-color: #333; margin: 20px 0;">
    <div>Your API Key:</div>
    <pre style="background: #111; padding: 10px; overflow-x: auto;">{apiKey}</pre>
    <br>
    <div>Keep this key secure. It authenticates all your requests.</div>

</div>