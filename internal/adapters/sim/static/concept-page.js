import './engine/sim-player.js';
import { concepts } from './registry.js';

// URL shape: /sim/{topic}/{id}
const [, , topic, id] = location.pathname.split('/');
const entry = concepts.find((c) => c.topic === topic && c.id === id);

const title = entry ? entry.title : 'Unknown concept';
document.getElementById('title').textContent = title;
document.title = `${title} – babylon sim`;

const player = document.createElement('sim-player');
player.setAttribute('concept', entry ? entry.id : '');
document.getElementById('host').append(player);
