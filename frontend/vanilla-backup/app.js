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

    // Curator Request Logic
    const sendRequestBtn = document.getElementById('send-request-btn');
    const requestInput = document.getElementById('curator-request-input');
    if (sendRequestBtn && requestInput) {
        sendRequestBtn.addEventListener('click', async () => {
            const prompt = requestInput.value.trim();
            if (!prompt) return;

            // Clear previous search overlay steps
            const thinkingSteps = document.getElementById('thinking-steps');
            if (thinkingSteps) thinkingSteps.innerHTML = '';

            try {
                const res = await fetch('/api/request', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ prompt })
                });
                if (res.ok) {
                    requestInput.value = '';
                    mockLoadData(); // Start polling status
                }
            } catch (err) {
                console.error("Failed to send request:", err);
            }
        });
    }

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
            geminiKey: document.getElementById('gemini-key').value,
            llmProvider: document.getElementById('llm-provider').value,
            preferredLanguage: document.getElementById('preferred-language').value,
            strictLanguage: document.getElementById('strict-language').checked,
            radarrQualityProfileId: parseInt(document.getElementById('radarr-profile').value) || 1,
            minRating: parseFloat(document.getElementById('min-rating').value) || 6.5,
            discoveryMood: document.getElementById('discovery-mood').value,
            discoveryPersona: document.getElementById('discovery-persona').value,
            discordWebhookUrl: document.getElementById('discord-webhook').value,
            excludedEras: document.getElementById('excluded-eras').value,
            excludedGenres: document.getElementById('excluded-genres').value,
            suggestInLibrary: document.getElementById('suggest-in-library').checked,
            noteToCurator: document.getElementById('note-to-curator').value
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

    // Test LLM Button logic
    const testBtn = document.getElementById('test-llm-btn');
    const testResult = document.getElementById('test-result');

    if (testBtn) {
        testBtn.addEventListener('click', async () => {
            const provider = document.getElementById('llm-provider').value;
            const apiKey = document.getElementById('gemini-key').value;

            testBtn.disabled = true;
            testBtn.innerText = 'Testing...';
            testResult.style.display = 'none';
            testResult.className = '';

            try {
                const res = await fetch('/api/test-llm', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ provider, apiKey })
                });
                
                const data = await res.json();
                testResult.style.display = 'block';
                
                if (res.ok && data.status === 'success') {
                    testResult.innerHTML = `<strong>✅ Success!</strong><br>${data.message}<br>Suggested movie: <em>${data.movie}</em>`;
                    testResult.classList.add('test-success');
                } else {
                    testResult.innerHTML = `<strong>❌ Connection Failed</strong><br>${data.message || 'Unknown error'}`;
                    testResult.classList.add('test-error');
                }
            } catch (error) {
                testResult.style.display = 'block';
                testResult.innerHTML = `<strong>❌ Error</strong><br>Could not reach the backend server.`;
                testResult.classList.add('test-error');
            } finally {
                testBtn.disabled = false;
                testBtn.innerText = 'Test LLM Connection';
            }
        });
    }

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
                { id: 'llm-provider', val: cfg.llmProvider, env: cfg.llmProviderFromEnv },
                { id: 'preferred-language', val: cfg.preferredLanguage, env: false },
                { id: 'strict-language', val: cfg.strictLanguage, env: false, type: 'checkbox' },
                { id: 'radarr-profile', val: cfg.radarrQualityProfileId, env: false, type: 'select' },
                { id: 'min-rating', val: cfg.minRating, env: false },
                { id: 'discovery-mood', val: cfg.discoveryMood, env: false, type: 'select' },
                { id: 'discovery-persona', val: cfg.discoveryPersona, env: false, type: 'select' },
                { id: 'discord-webhook', val: cfg.discordWebhookUrl, env: false },
                { id: 'excluded-eras', val: cfg.excludedEras, env: false },
                { id: 'excluded-genres', val: cfg.excludedGenres, env: false },
                { id: 'suggest-in-library', val: cfg.suggestInLibrary, env: false, type: 'checkbox' },
                { id: 'note-to-curator', val: cfg.noteToCurator, env: false },
            ];
            
            fields.forEach(f => {
                const el = document.getElementById(f.id);
                if (!el) return; // Skip if element doesn't exist

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
                } else if (f.val !== undefined) {
                    if (f.type === 'checkbox') {
                        el.checked = f.val;
                    } else if (f.type === 'select') {
                        // Select logic is handled after population
                    } else {
                        el.value = f.val;
                    }
                }
            });
        }
    } catch (e) {
        console.error("Failed to load config:", e);
    }

    // New: Fetch and populate Radarr profiles
    await fetchRadarrProfiles();
}

async function fetchRadarrProfiles() {
    const profileSelect = document.getElementById('radarr-profile');
    try {
        const res = await fetch('/api/radarr/profiles');
        if (res.ok) {
            const profiles = await res.json();
            if (profileSelect) {
                profileSelect.innerHTML = profiles.map(p => `
                    <option value="${p.id}">${p.name}</option>
                `).join('');
            }
            
            // Re-select current value from config if available
            const currentCfg = await (await fetch('/api/config')).json();
            if (currentCfg.radarrQualityProfileId) {
                profileSelect.value = currentCfg.radarrQualityProfileId;
            }
        }
    } catch (e) {
        console.error("Failed to fetch Radarr profiles:", e);
    }
}

async function mockLoadData() {
    const currentMovieArea = document.getElementById('current-movie');
    const downloadStatus = document.getElementById('download-status');

    try {
        const stateRes = await fetch('/api/state');
        if (stateRes.ok) {
            const state = await stateRes.json();
            
            // Update Engine Status UI
            const statusEl = document.getElementById('engine-status');
            const triggerBtn = document.getElementById('trigger-btn');
            const searchBtn = document.getElementById('search-btn');
            
            if (statusEl) {
                statusEl.innerText = state.status || "";
            }

            // Update Cinematic Compass (Spectrum)
            if (state.cinematicSpectrum) {
                renderSpectrum(state.cinematicSpectrum);
            }

            // Update Taste Interpretation
            const tasteEl = document.getElementById('taste-profile-content');
            if (tasteEl) {
                const summary = state.tasteProfile || "Establishing your cinematic taste... Check back after a few searches.";
                tasteEl.innerHTML = summary.split('\n\n').map(p => `<p style="margin-bottom: 1rem;">${p.replace(/\n/g, '<br>')}</p>`).join('');
            }

            if (state.isRunning) {
                if (triggerBtn) {
                    triggerBtn.disabled = true;
                    triggerBtn.style.opacity = '0.5';
                    triggerBtn.innerText = 'Searching...';
                }
                if (searchBtn) {
                    searchBtn.disabled = true;
                    searchBtn.style.opacity = '0.5';
                }
                // Poll for updates every 2 seconds if running
                if (!window.statusInterval) {
                    window.statusInterval = setInterval(mockLoadData, 2000);
                }
                
                // Agent Thinking Overlay Logic
                const thinkingOverlay = document.getElementById('thinking-overlay');
                const thinkingSteps = document.getElementById('thinking-steps');
                if (thinkingOverlay && thinkingSteps) {
                    thinkingOverlay.style.display = 'flex';
                    if (state.status) {
                        const newStep = document.createElement('div');
                        newStep.innerText = state.status;
                        // Avoid duplicates
                        if (!thinkingSteps.innerText.includes(state.status)) {
                            thinkingSteps.appendChild(newStep);
                        }
                    }
                }
            } else {
                const thinkingOverlay = document.getElementById('thinking-overlay');
                if (thinkingOverlay) thinkingOverlay.style.display = 'none';

                if (triggerBtn) {
                    triggerBtn.disabled = false;
                    triggerBtn.style.opacity = '1';
                    triggerBtn.innerHTML = '<span>🎲</span> SPIN THE DIAL';
                }
                if (searchBtn) {
                    searchBtn.disabled = false;
                    searchBtn.style.opacity = '1';
                }
                if (window.statusInterval) {
                    clearInterval(window.statusInterval);
                    window.statusInterval = null;
                }
            }

            if (state && state.lastMovieTitle) {
                const posterUrl = state.lastMoviePosterPath ? `https://image.tmdb.org/t/p/w500${state.lastMoviePosterPath}` : 'https://via.placeholder.com/200x300?text=No+Poster';
                
                let actionButtons = `
                    <div class="button-group" style="display: flex; gap: 1rem; margin-top: 1.5rem;">
                        <button class="btn-primary" style="width: auto;" onclick="triggerJob()">SPIN AGAIN</button>
                    </div>
                `;
                
                if (state.isSuggested) {
                    actionButtons = `
                        <div style="display: flex; gap: 1rem; margin-top: 1.5rem; justify-content: flex-start; flex-wrap: wrap; position: relative;">
                            <button class="btn-primary" style="background: #006400; box-shadow: 0 4px 0 #004d00; width: auto;" onclick="addMovie(${state.lastMovieId})">ADD TO REEL</button>
                            <button class="btn-secondary" style="width: auto; background: #c5a059; border-color: #a88a4d; color: #2c1e14;" onclick="endorseMovie(${state.lastMovieId})">ENDORSE</button>
                            <button class="btn-secondary" style="width: auto;" onclick="rejectMovie(${state.lastMovieId})">DISMISS</button>
                            <button class="btn-secondary" style="width: auto;" onclick="triggerSearch()">SEARCH AIRWAVES</button>
                            <button class="btn-icon" style="position: absolute; top: -10px; right: 0; background: rgba(0,0,0,0.5); border: 1px solid #333; color: #666; width: 25px; height: 25px; border-radius: 50%; cursor: pointer;" onclick="clearSuggestion()">✕</button>
                        </div>
                    `;
                }

                const reasoningHtml = state.lastMovieReasoning ? 
                    state.lastMovieReasoning.split('\n\n').map(p => `<p style="margin-bottom: 1rem; font-style: italic;">"${p.trim()}"</p>`).join('') : '';

                currentMovieArea.innerHTML = `
                    <div class="movie-card-active fade-in">
                        <img class="movie-poster" src="${posterUrl}" alt="${state.lastMovieTitle}">
                        <div class="movie-info">
                            <div style="display: flex; justify-content: space-between; align-items: flex-start;">
                                <div style="flex: 1;">
                                    <h3 style="margin: 0;">${state.lastMovieTitle}</h3>
                                    ${state.lastMoviePathTheme ? `<div style="font-family: var(--font-mono); color: var(--accent-color); font-size: 0.9rem; margin-top: 0.3rem;">PATH: ${state.lastMoviePathTheme.toUpperCase()}</div>` : ''}
                                </div>
                                ${state.isSuggested ? '<span style="font-family: var(--font-mono); background: var(--accent-color); color: black; padding: 2px 8px; font-size: 0.7rem; font-weight: bold; border-radius: 2px; margin-left: 1rem;">BROADCAST SIGNAL</span>' : ''}
                            </div>
                            <div class="movie-meta" style="font-family: var(--font-mono); margin: 0.5rem 0; color: var(--accent-color);">
                                <span>RATING: ${Number(state.lastMovieRating).toFixed(1)}/10</span> • 
                                <a href="https://www.themoviedb.org/movie/${state.lastMovieId}" target="_blank" style="color: var(--text-secondary); text-decoration: underline;">TMDB_REF</a>
                            </div>
                            
                            ${state.lastMovieReasoning ? `
                                <div style="background: rgba(197, 160, 89, 0.1); border-left: 3px solid var(--accent-color); padding: 1.5rem; margin: 1.5rem 0; color: var(--paper-color); line-height: 1.6;">
                                    <div style="font-family: var(--font-mono); font-size: 0.7rem; color: var(--accent-color); margin-bottom: 1rem; letter-spacing: 1px; font-weight: bold;">CURATOR'S RESEARCH:</div>
                                    ${reasoningHtml}
                                </div>
                            ` : ''}
                            
                            <p style="margin-bottom: 1.5rem; font-size: 1rem; color: #aaa;">${state.lastMovieOverview}</p>
                            
                            ${state.lastMovieTrailerKey ? `
                                <div style="margin-bottom: 1.5rem; border: 4px solid #1a1a1a; border-radius: 8px; overflow: hidden; box-shadow: inset 0 0 15px rgba(0,0,0,1);">
                                    <div style="position: relative; padding-bottom: 56.25%; height: 0;">
                                        <iframe style="position: absolute; top: 0; left: 0; width: 100%; height: 100%; border: none; filter: sepia(0.2) contrast(1.1);" 
                                            src="https://www.youtube.com/embed/${state.lastMovieTrailerKey}?autoplay=0&controls=1&showinfo=0&modestbranding=1" 
                                            allowfullscreen>
                                        </iframe>
                                    </div>
                                </div>
                            ` : ''}
                            
                            ${actionButtons}
                        </div>
                    </div>
                `;
            } else if (state.isRunning) {
                currentMovieArea.innerHTML = `
                    <div style="text-align: center; padding: 4rem;" class="fade-in">
                        <h3 style="color: var(--paper-color); font-family: var(--font-heading); font-size: 2.5rem; margin-bottom: 1rem;" class="loading-pulse">TUNING SIGNAL...</h3>
                        <p style="color: var(--text-secondary); font-family: var(--font-mono); margin-bottom: 2rem;">BROADCAST IN PROGRESS</p>
                        <div id="engine-status" style="margin-top: 2rem; color: var(--accent-color); font-family: var(--font-mono); font-weight: bold;">${state.status || ""}</div>
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
        const downloadRes = await fetch('/api/downloads');
        if (downloadRes.ok) {
            const queue = await downloadRes.json();
            if (queue && queue.length > 0) {
                downloadStatus.innerHTML = queue.map(q => {
                    const percent = ((q.size - q.sizeleft) / q.size) * 100 || 0;
                    return `
                        <div class="status-item" style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.5rem; font-family: var(--font-mono); color: var(--text-secondary);">
                            <span>REEL: ${q.title}</span>
                            <span>${percent.toFixed(1)}%</span>
                        </div>
                        <div style="width: 100%; height: 10px; background: #333; border: 2px solid #555; overflow: hidden; margin-bottom: 1rem;">
                            <div style="width: ${percent}%; height: 100%; background: #006400;"></div>
                        </div>
                    `;
                }).join('');
            } else {
                downloadStatus.innerHTML = '<p style="font-family: var(--font-mono); color: var(--text-secondary); text-align: center;">NO ACTIVE REELS IN TRANSIT</p>';
            }
        }
    } catch (e) {
        console.error("Failed to fetch downloads:", e);
    }
}

function renderSpectrum(spectrum) {
    const container = document.getElementById('spectrum-container');
    if (!container) return;

    if (!spectrum || spectrum.length === 0) {
        container.innerHTML = '<div class="loading-overlay">AWAITING EXPERT ANALYSIS...</div>';
        return;
    }

    container.innerHTML = spectrum.map(dim => {
        const total = dim.strengthA + dim.strengthB;
        const widthA = total > 0 ? (dim.strengthA / 20) * 100 : 0;
        const widthB = total > 0 ? (dim.strengthB / 20) * 100 : 0;
        
        // Net indicator: 0 strengthA = 0%, 10 strengthA & 10 strengthB = 50%, 0 strengthB = 100%
        // Actually, let's just use the balance of A vs B
        const net = total > 0 ? (dim.strengthA / total) : 0.5;
        // Net indicator position: A is left (0%), B is right (100%)
        // So strengthA=10, strengthB=0 -> pos=0%
        // strengthA=0, strengthB=10 -> pos=100%
        const pos = total > 0 ? (dim.strengthB / total) * 100 : 50;

        return `
            <div class="spectrum-item">
                <div class="spectrum-label">
                    <span>${dim.poleA} (${dim.strengthA})</span>
                    <span>${dim.poleB} (${dim.strengthB})</span>
                </div>
                <div class="spectrum-meter-container" title="${dim.name}">
                    <div class="strength-bar-a" style="width: ${widthA}%"></div>
                    <div style="flex: 1; background: rgba(0,0,0,0.1)"></div>
                    <div class="strength-bar-b" style="width: ${widthB}%"></div>
                    <div class="net-indicator" style="left: calc(${pos}% - 2px)"></div>
                </div>
                <!-- <div style="text-align: center; font-size: 0.6rem; font-family: var(--font-mono); color: var(--sepia-med); margin-top: 2px;">${dim.name}</div> -->
            </div>
        `;
    }).join('');
}


async function showDefaultMovie(container) {
    try {
        const res = await fetch('/api/schedule');
        const data = res.ok ? await res.json() : { nextRun: 'Friday at 6:00 PM' };
        
        container.innerHTML = `
            <div style="text-align: center; padding: 4rem;" class="fade-in">
                <h3 id="next-run-time" style="color: var(--paper-color); font-family: var(--font-heading); font-size: 2.5rem; margin-bottom: 1rem;">TUNED TO: ${data.nextRun}</h3>
                <p style="color: var(--text-secondary); font-family: var(--font-mono); margin-bottom: 2rem;">WAITING FOR NEXT BROADCAST</p>
                <div class="button-group" style="display: flex; gap: 1.5rem; justify-content: center; flex-wrap: wrap;">
                    <button id="trigger-btn" class="btn-primary" onclick="triggerJob()" style="padding: 1rem 2rem;">
                        <span>🎲</span> SPIN THE DIAL
                    </button>
                    <button id="search-btn" class="btn-secondary" style="padding: 1rem 2rem; background: #666; border: 2px solid #444; color: white;" onclick="triggerSearch()">
                        <span>🔍</span> SEARCH AIRWAVES
                    </button>
                </div>
                <div id="engine-status" style="margin-top: 2rem; color: var(--accent-color); font-family: var(--font-mono); font-weight: bold;"></div>
            </div>
        `;
    } catch (e) {
        // Fallback already handled or not critical
    }
}

async function triggerJob() {
    try {
        const res = await fetch('/api/trigger', { method: 'POST' });
        if (res.ok) {
            mockLoadData();
        } else {
            const data = await res.json();
            alert('Failed: ' + (data.status || 'Unknown error'));
        }
    } catch (e) {
        console.error("Failed to trigger job:", e);
    }
}

async function triggerSearch() {
    try {
        const res = await fetch('/api/search', { method: 'POST' });
        if (res.ok) {
            mockLoadData();
        } else {
            const data = await res.json();
            alert('Failed: ' + (data.status || 'Unknown error'));
        }
    } catch (e) {
        console.error("Failed to trigger search:", e);
    }
}

async function addMovie(tmdbId) {
    try {
        const res = await fetch('/api/add', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ tmdbId: tmdbId })
        });
        if (res.ok) {
            mockLoadData();
        } else {
            const data = await res.json();
            alert('Failed to add movie: ' + (data.status || 'Unknown error'));
        }
    } catch (e) {
        console.error("Failed to add movie:", e);
    }
}

async function rejectMovie(tmdbId) {
    const reason = window.prompt('Why are you rejecting this suggestion? (e.g., "Too violent", "Seen it", "Not my mood"). Your reason will help the curator learn.');
    if (reason === null) return; // User cancelled
    
    try {
        const res = await fetch('/api/reject', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ tmdbId: tmdbId, reason: reason })
        });
        if (res.ok) {
            mockLoadData();
        } else {
            const data = await res.json();
            alert('Failed to reject movie: ' + (data.status || 'Unknown error'));
        }
    } catch (e) {
        console.error("Failed to reject movie:", e);
    }
}

async function clearSuggestion() {
    try {
        const res = await fetch('/api/clear-suggestion', { method: 'POST' });
        if (res.ok) {
            mockLoadData();
        } else {
            console.error('Failed to clear suggestion');
        }
    } catch (e) {
        console.error("Failed to clear suggestion:", e);
    }
}

async function endorseMovie(tmdbId) {
    try {
        const res = await fetch('/api/endorse', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ tmdbId: tmdbId })
        });
        if (res.ok) {
            mockLoadData();
        } else {
            const data = await res.json();
            alert('Failed to endorse movie: ' + (data.status || 'Unknown error'));
        }
    } catch (e) {
        console.error("Failed to endorse movie:", e);
    }
}

