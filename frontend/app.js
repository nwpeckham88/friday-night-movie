document.addEventListener('DOMContentLoaded', () => {
    // Navigation Logic
    const navDashboard = document.getElementById('nav-dashboard');
    const navSettings = document.getElementById('nav-settings');
    const viewDashboard = document.getElementById('view-dashboard');
    const viewSettings = document.getElementById('view-settings');

    navDashboard.addEventListener('click', () => {
        navDashboard.classList.add('active');
        navSettings.classList.remove('active');
        viewDashboard.classList.add('active-view');
        viewSettings.classList.remove('active-view');
    });

    navSettings.addEventListener('click', () => {
        navSettings.classList.add('active');
        navDashboard.classList.remove('active');
        viewSettings.classList.add('active-view');
        viewDashboard.classList.remove('active-view');
    });

    // Settings Form Submit
    const settingsForm = document.getElementById('settings-form');
    settingsForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        
        const config = {
            jellyfinUrl: document.getElementById('jellyfin-url').value,
            jellyfinKey: document.getElementById('jellyfin-key').value,
            radarrUrl: document.getElementById('radarr-url').value,
            radarrKey: document.getElementById('radarr-key').value,
            tmdbKey: document.getElementById('tmdb-key').value,
            geminiKey: document.getElementById('gemini-key').value
        };

        // In a real app, send to backend API
        console.log('Saving config to backend...', config);
        
        try {
            const res = await fetch('/api/config', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(config)
            });
            if (res.ok) {
                alert('Settings saved successfully!');
            } else {
                alert('Failed to save settings.');
            }
        } catch (error) {
            console.error('API not ready yet', error);
            alert('Settings saved (mocked)!');
        }
    });

    // Load initial data
    loadConfig();
    mockLoadData();
});

async function loadConfig() {
    try {
        const res = await fetch('/api/config');
        if (res.ok) {
            const cfg = await res.json();
            const fields = [
                { id: 'jellyfin-url', val: cfg.jellyfinUrl, env: cfg.jellyfinUrlFromEnv },
                { id: 'jellyfin-key', val: cfg.jellyfinKey, env: cfg.jellyfinKeyFromEnv },
                { id: 'radarr-url', val: cfg.radarrUrl, env: cfg.radarrUrlFromEnv },
                { id: 'radarr-key', val: cfg.radarrKey, env: cfg.radarrKeyFromEnv },
                { id: 'tmdb-key', val: cfg.tmdbKey, env: cfg.tmdbKeyFromEnv },
                { id: 'gemini-key', val: cfg.geminiKey, env: cfg.geminiKeyFromEnv },
            ];
            
            fields.forEach(f => {
                const el = document.getElementById(f.id);
                if (f.env) {
                    el.value = f.val;
                    el.placeholder = 'Loaded from environment';
                    el.disabled = true;
                    el.style.opacity = '0.5';
                    el.style.cursor = 'not-allowed';
                    const label = document.querySelector(`label[for="${f.id}"]`);
                    if (label && !label.innerHTML.includes('Loaded from environment')) {
                        label.innerHTML += ' <span style="color: var(--success-color); font-size: 0.75rem; margin-left: 0.5rem; padding: 0.1rem 0.4rem; border-radius: 4px; border: 1px solid var(--success-color);">Loaded from environment</span>';
                    }
                } else if (f.val) {
                    el.value = f.val;
                }
            });
        }
    } catch (e) {
        console.error("Failed to load config:", e);
    }
}

async function mockLoadData() {
    const currentMovieArea = document.getElementById('current-movie');
    const historyList = document.getElementById('history-list');
    const downloadStatus = document.getElementById('download-status');

    try {
        const stateRes = await fetch('/api/state');
        if (stateRes.ok) {
            const state = await stateRes.json();
            if (state && state.lastMovieTitle) {
                const posterUrl = state.lastMoviePosterPath ? `https://image.tmdb.org/t/p/w500${state.lastMoviePosterPath}` : 'https://via.placeholder.com/200x300?text=No+Poster';
                currentMovieArea.innerHTML = `
                    <img class="movie-poster fade-in" src="${posterUrl}" alt="${state.lastMovieTitle}">
                    <div class="movie-info fade-in" style="animation-delay: 0.2s;">
                        <h3 style="color: var(--text-primary); font-size: 1.5rem;">${state.lastMovieTitle}</h3>
                        <p>${state.lastMovieOverview}</p>
                        <div class="movie-meta">
                            <span>⭐ ${state.lastMovieRating}/10</span>
                        </div>
                        <button class="btn-primary" style="margin-top: 1rem; width: auto;" onclick="triggerJob()">Trigger Next Movie Now</button>
                    </div>
                `;
            } else {
                showDefaultMovie(currentMovieArea);
            }
        } else {
            showDefaultMovie(currentMovieArea);
        }
    } catch (e) {
        console.error("Failed to fetch state:", e);
        showDefaultMovie(currentMovieArea);
    }

    try {
        const historyRes = await fetch('/api/history');
        if (historyRes.ok) {
            const movies = await historyRes.json();
            if (movies && movies.length > 0) {
                historyList.innerHTML = movies.slice(0, 5).map(m => `
                    <li><span>${m.Name}</span> <span style="color: var(--text-secondary)">Archived</span></li>
                `).join('');
            } else {
                historyList.innerHTML = '<li><span>No history found.</span></li>';
            }
        }
    } catch (e) {
        console.error("Failed to fetch history:", e);
    }

    try {
        const downloadRes = await fetch('/api/downloads');
        if (downloadRes.ok) {
            const queue = await downloadRes.json();
            if (queue && queue.length > 0) {
                downloadStatus.innerHTML = queue.map(q => {
                    const percent = ((q.size - q.sizeleft) / q.size) * 100 || 0;
                    return `
                        <div class="status-item" style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem;">
                            <span>${q.title}</span>
                            <span style="color: var(--accent-color);">${q.status}... ${percent.toFixed(1)}%</span>
                        </div>
                        <div style="width: 100%; height: 6px; background: rgba(255,255,255,0.1); border-radius: 3px; overflow: hidden; margin-bottom: 1rem;">
                            <div style="width: ${percent}%; height: 100%; background: var(--success-color); box-shadow: 0 0 10px var(--success-color);"></div>
                        </div>
                    `;
                }).join('');
            } else {
                downloadStatus.innerHTML = '<div class="status-item">No active downloads</div>';
            }
        }
    } catch (e) {
        console.error("Failed to fetch downloads:", e);
    }
}

async function showDefaultMovie(container) {
    try {
        const res = await fetch('/api/schedule');
        const data = res.ok ? await res.json() : { nextRun: 'Friday at 6:00 PM' };
        
        container.innerHTML = `
            <div style="text-align: center; padding: 2rem;" class="fade-in">
                <h3 id="next-run-time" style="color: var(--text-primary); margin-bottom: 1rem;">Next Selection: ${data.nextRun}</h3>
                <p style="color: var(--text-secondary); margin-bottom: 2rem;">The Friday Night Movie engine hasn't selected a movie yet.</p>
                <button class="btn-primary" onclick="triggerJob()" style="display: flex; align-items: center; justify-content: center; gap: 0.5rem; margin: 0 auto;">
                    <span>🎲</span> I'm feeling lucky
                </button>
            </div>
        `;
    } catch (e) {
        container.innerHTML = `
            <div style="text-align: center; padding: 2rem;" class="fade-in">
                <h3 id="next-run-time" style="color: var(--text-primary); margin-bottom: 1rem;">Next Selection: Friday at 6:00 PM</h3>
                <p style="color: var(--text-secondary); margin-bottom: 2rem;">The Friday Night Movie engine hasn't selected a movie yet.</p>
                <button class="btn-primary" onclick="triggerJob()" style="display: flex; align-items: center; justify-content: center; gap: 0.5rem; margin: 0 auto;">
                    <span>🎲</span> I'm feeling lucky
                </button>
            </div>
        `;
    }
}

async function triggerJob() {
    try {
        const container = document.getElementById('current-movie');
        container.innerHTML = '<div class="loading-pulse">Selecting a movie now...</div>';
        
        const res = await fetch('/api/trigger', { method: 'POST' });
        if (res.ok) {
            // Wait a moment for the backend job to complete
            setTimeout(mockLoadData, 3000);
        } else {
            alert('Failed to trigger the movie engine.');
            showDefaultMovie(container);
        }
    } catch (e) {
        console.error("Failed to trigger job:", e);
        alert('Failed to trigger the movie engine.');
        showDefaultMovie(document.getElementById('current-movie'));
    }
}
