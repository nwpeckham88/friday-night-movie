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
            tmdbKey: document.getElementById('tmdb-key').value
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

    // Load initial data (mocked for now, will connect to Go backend)
    setTimeout(() => {
        mockLoadData();
    }, 1500);
});

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
