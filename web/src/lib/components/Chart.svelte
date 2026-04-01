<script lang="ts">
	import { onMount } from 'svelte';
	import {
		Chart,
		LineController,
		LineElement,
		PointElement,
		LinearScale,
		TimeScale,
		Filler,
		Tooltip
	} from 'chart.js';
	import 'chartjs-adapter-date-fns';

	Chart.register(LineController, LineElement, PointElement, LinearScale, TimeScale, Filler, Tooltip);

	interface Props {
		data: Array<{ t: number; v: number }>;
		label?: string;
		height?: number;
		type?: 'line' | 'scatter';
	}

	let { data, label = '', height = 220, type = 'line' }: Props = $props();

	let canvas: HTMLCanvasElement;
	let chart: Chart | null = null;

	function makeDataset(d: typeof data, t: 'line' | 'scatter') {
		return {
			label,
			data: d.map((s) => ({ x: s.t * 1000, y: s.v })),
			borderColor: '#ffffff',
			backgroundColor: '#ffffff',
			borderWidth: t === 'scatter' ? 0 : 2,
			pointRadius: t === 'scatter' ? 3 : 0,
			pointHoverRadius: 5,
			fill: false,
			tension: 0,
			showLine: t === 'line'
		};
	}

	onMount(() => {
		chart = new Chart(canvas, {
			type: 'line',
			data: { datasets: [makeDataset(data, type)] },
			options: {
				responsive: true,
				maintainAspectRatio: false,
				animation: false,
				scales: {
					x: {
						type: 'time',
						time: { unit: 'minute', tooltipFormat: 'HH:mm:ss' },
						grid: { color: '#1e1e1e' },
						ticks: { color: '#888888', font: { family: 'IBM Plex Mono', size: 11 }, maxTicksLimit: 8 },
						border: { color: '#333333' }
					},
					y: {
						grid: { color: '#1e1e1e' },
						ticks: { color: '#888888', font: { family: 'IBM Plex Mono', size: 11 } },
						border: { color: '#333333' }
					}
				},
				plugins: {
					legend: { display: false },
					tooltip: {
						backgroundColor: '#000000',
						borderColor: '#ffffff',
						borderWidth: 1,
						titleColor: '#888888',
						bodyColor: '#ffffff',
						titleFont: { family: 'IBM Plex Mono', size: 11 },
						bodyFont: { family: 'IBM Plex Mono', size: 12 },
						callbacks: {
							title: (items) =>
								new Date(items[0].parsed.x!).toLocaleTimeString(),
							label: (item) =>
								`${label ? label + ': ' : ''}${(item.parsed.y as number).toFixed(4)}`
						}
					}
				}
			}
		});

		return () => chart?.destroy();
	});

	$effect(() => {
		if (!chart) return;
		chart.data.datasets[0] = makeDataset(data, type);
		chart.update('none');
	});
</script>

<div style="position: relative; height: {height}px;">
	<canvas bind:this={canvas}></canvas>
</div>
