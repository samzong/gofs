(function() {
    'use strict';
    const state = {
        viewMode: localStorage.getItem('viewMode') || 'grid',
        theme: localStorage.getItem('theme') || 'light',
        layoutMode: localStorage.getItem('layoutMode') || 'centered',
        sortBy: 'name',
        sortOrder: 'asc',
        searchQuery: '',
        uploadProgress: 0,
        uploadXHR: null,
        csrfToken: null,
        selectedFiles: new Set(),
        isSelectionMode: false,
        lastSelectedIndex: -1
    };
    const elements = {
        html: document.documentElement,
        body: document.body,
        fileContainer: document.getElementById('fileContainer'),
        searchInput: document.getElementById('searchInput'),
        viewToggle: document.getElementById('viewToggle'),
        themeToggle: document.getElementById('themeToggle'),
        layoutToggle: document.getElementById('layoutToggle'),
        uploadInput: document.getElementById('uploadInput'),
        dropZone: document.getElementById('dropZone'),
        uploadProgress: document.getElementById('uploadProgress'),
        progressFill: document.getElementById('progressFill'),
        uploadPercent: document.getElementById('uploadPercent'),
        uploadSpeed: document.getElementById('uploadSpeed'),
        uploadFilename: document.getElementById('uploadFilename'),
        uploadCancel: document.getElementById('uploadCancel'),
        previewModal: document.getElementById('previewModal'),
        previewTitle: document.getElementById('previewTitle'),
        previewBody: document.getElementById('previewBody'),
        previewClose: document.getElementById('previewClose'),
        newFolderBtn: document.getElementById('newFolderBtn')
    };
    function init() {
        applyTheme(state.theme);
        applyViewMode(state.viewMode);
        applyLayoutMode(state.layoutMode);
        setupEventListeners();
        setupDragAndDrop();
        setupKeyboardShortcuts();
        initializeSelection();
        fetchCSRFToken();
    }

    function fetchCSRFToken() {
        fetch('/api/csrf')
            .then(response => response.json())
            .then(data => {
                state.csrfToken = data.token;
            })
            .catch(err => {
                console.error('Failed to fetch CSRF token:', err);
            });
    }

    function getCSRFToken() {
        if (!state.csrfToken) {
            const xhr = new XMLHttpRequest();
            xhr.open('GET', '/api/csrf', false);
            xhr.send();
            if (xhr.status === 200) {
                const data = JSON.parse(xhr.responseText);
                state.csrfToken = data.token;
            }
        }
        return state.csrfToken;
    }

    function applyTheme(theme) {
        state.theme = theme;
        elements.html.setAttribute('data-theme', theme);
        localStorage.setItem('theme', theme);
        updateThemeToggleIcon(theme);
    }

    function toggleTheme() {
        const newTheme = state.theme === 'light' ? 'dark' : 'light';
        applyTheme(newTheme);
    }

    function applyViewMode(mode) {
        state.viewMode = mode;
        elements.fileContainer.className = `file-container ${mode}-view`;
        localStorage.setItem('viewMode', mode);
    }

    function toggleViewMode() {
        const newMode = state.viewMode === 'grid' ? 'list' : 'grid';
        applyViewMode(newMode);
        updateViewToggleIcon(newMode);
    }

    function updateViewToggleIcon(mode) {
        if (!elements.viewToggle) return;
        const gridIcon = elements.viewToggle.querySelector('.view-grid');
        const listIcon = elements.viewToggle.querySelector('.view-list');
        
        if (mode === 'grid') {
            gridIcon.style.display = 'block';
            listIcon.style.display = 'none';
            elements.viewToggle.title = 'Switch to List View';
        } else {
            gridIcon.style.display = 'none';
            listIcon.style.display = 'block';
            elements.viewToggle.title = 'Switch to Grid View';
        }
    }

    function updateThemeToggleIcon(theme) {
        if (!elements.themeToggle) return;
        const sunIcon = elements.themeToggle.querySelector('.theme-sun');
        const moonIcon = elements.themeToggle.querySelector('.theme-moon');
        
        if (theme === 'light') {
            sunIcon.style.display = 'block';
            moonIcon.style.display = 'none';
            elements.themeToggle.title = 'Switch to Dark Theme';
        } else {
            sunIcon.style.display = 'none';
            moonIcon.style.display = 'block';
            elements.themeToggle.title = 'Switch to Light Theme';
        }
    }

    function applyLayoutMode(mode) {
        state.layoutMode = mode;
        elements.body.className = elements.body.className
            .replace(/layout-\w+/g, '') + ` layout-${mode}`;
        localStorage.setItem('layoutMode', mode);
        updateLayoutToggleIcon(mode);
    }

    function toggleLayoutMode() {
        const newMode = state.layoutMode === 'centered' ? 'fullwidth' : 'centered';
        applyLayoutMode(newMode);
    }

    function updateLayoutToggleIcon(mode) {
        if (!elements.layoutToggle) return;
        const centeredIcon = elements.layoutToggle.querySelector('.layout-centered');
        const fullwidthIcon = elements.layoutToggle.querySelector('.layout-fullwidth');
        
        if (mode === 'centered') {
            centeredIcon.style.display = 'block';
            fullwidthIcon.style.display = 'none';
            elements.layoutToggle.title = 'Switch to Fullwidth Layout';
        } else {
            centeredIcon.style.display = 'none';
            fullwidthIcon.style.display = 'block';
            elements.layoutToggle.title = 'Switch to Centered Layout';
        }
    }

    function handleSearch() {
        const query = elements.searchInput.value.toLowerCase();
        state.searchQuery = query;
        
        const fileItems = elements.fileContainer.querySelectorAll('.file-item:not(.file-item-parent)');
        let visibleCount = 0;
        
        fileItems.forEach(item => {
            const name = item.dataset.name ? item.dataset.name.toLowerCase() : '';
            const matches = !query || name.includes(query);
            item.style.display = matches ? '' : 'none';
            if (matches) visibleCount++;
        });
        
        const fileCount = document.querySelector('.file-count');
        if (fileCount) {
            const total = fileItems.length;
            if (query) {
                fileCount.textContent = `${visibleCount} of ${total} items`;
            } else {
                fileCount.textContent = `${total} items`;
            }
        }
    }

    function handleFileSelect(files) {
        if (files.length === 0) return;
        
        const file = files[0];
        uploadFile(file);
    }

    function uploadFile(file) {
        const maxSize = 100 * 1024 * 1024;
        if (file.size > maxSize) {
            showNotification('File too large. Maximum size is 100MB.', 'error');
            return;
        }

        elements.uploadProgress.style.display = 'block';
        elements.uploadFilename.textContent = file.name;
        elements.progressFill.style.width = '0%';
        elements.uploadPercent.textContent = '0%';

        const formData = new FormData();
        formData.append('file', file);
        
        const csrfToken = getCSRFToken();
        if (csrfToken) {
            formData.append('csrf_token', csrfToken);
        }

        const xhr = new XMLHttpRequest();
        state.uploadXHR = xhr;

        const startTime = Date.now();
        let lastLoaded = 0;
        
        xhr.upload.addEventListener('progress', (e) => {
            if (e.lengthComputable) {
                const percentComplete = Math.round((e.loaded / e.total) * 100);
                state.uploadProgress = percentComplete;
                
                elements.progressFill.style.width = percentComplete + '%';
                elements.uploadPercent.textContent = percentComplete + '%';
                
                const elapsed = (Date.now() - startTime) / 1000;
                const bytesPerSecond = e.loaded / elapsed;
                const speed = formatSpeed(bytesPerSecond);
                elements.uploadSpeed.textContent = speed;
            }
        });

        xhr.addEventListener('load', () => {
            if (xhr.status === 200) {
                showNotification(`${file.name} uploaded successfully!`, 'success');
                setTimeout(() => location.reload(), 1000);
            } else {
                showNotification('Upload failed. Please try again.', 'error');
            }
            hideUploadProgress();
        });

        xhr.addEventListener('error', () => {
            showNotification('Upload error. Please check your connection.', 'error');
            hideUploadProgress();
        });

        xhr.addEventListener('abort', () => {
            showNotification('Upload cancelled.', 'info');
            hideUploadProgress();
        });

        xhr.open('POST', '/api/upload');
        if (csrfToken) {
            xhr.setRequestHeader('X-CSRF-Token', csrfToken);
        }
        xhr.send(formData);
        
        xhr.addEventListener('loadend', () => {
            fetchCSRFToken();
        });
    }

    function cancelUpload() {
        if (state.uploadXHR) {
            state.uploadXHR.abort();
            state.uploadXHR = null;
        }
    }

    function hideUploadProgress() {
        setTimeout(() => {
            elements.uploadProgress.style.display = 'none';
            state.uploadProgress = 0;
            state.uploadXHR = null;
        }, 500);
    }

    function setupDragAndDrop() {
        let dragCounter = 0;

        document.addEventListener('dragenter', (e) => {
            e.preventDefault();
            dragCounter++;
            elements.dropZone.classList.add('active');
        });

        document.addEventListener('dragleave', (e) => {
            e.preventDefault();
            dragCounter--;
            if (dragCounter === 0) {
                elements.dropZone.classList.remove('active');
            }
        });

        document.addEventListener('dragover', (e) => {
            e.preventDefault();
        });

        document.addEventListener('drop', (e) => {
            e.preventDefault();
            dragCounter = 0;
            elements.dropZone.classList.remove('active');
            
            const files = Array.from(e.dataTransfer.files);
            handleFileSelect(files);
        });
    }

    function previewFile(e) {
        const link = e.target.closest('.file-item');
        if (!link || link.classList.contains('file-item-parent')) return;
        
        const isFolder = link.dataset.type === 'folder';
        if (isFolder) return;
        
        const filename = link.dataset.name;
        const ext = filename.split('.').pop().toLowerCase();
        
        const imageExts = ['jpg', 'jpeg', 'png', 'gif', 'webp', 'svg'];
        const textExts = ['txt', 'md', 'json', 'js', 'css', 'html', 'xml', 'yml', 'yaml'];
        
        if (!imageExts.includes(ext) && !textExts.includes(ext)) {
            return;
        }
        
        e.preventDefault();
        
        elements.previewModal.style.display = 'flex';
        elements.previewTitle.textContent = filename;
        elements.previewBody.innerHTML = 'Loading...';
        
        const url = link.href;
        
        if (imageExts.includes(ext)) {
            elements.previewBody.innerHTML = `<img src="${url}" style="max-width: 100%; height: auto;">`;
        } else {
            fetch(url)
                .then(response => response.text())
                .then(text => {
                    const escaped = escapeHtml(text);
                    elements.previewBody.innerHTML = `<pre style="white-space: pre-wrap; word-wrap: break-word;">${escaped}</pre>`;
                })
                .catch(err => {
                    elements.previewBody.innerHTML = 'Error loading file.';
                });
        }
    }

    function createNewFolder() {
        const name = prompt('Enter folder name:');
        if (!name) return;
        
        if (!/^[a-zA-Z0-9_\-. ]+$/.test(name)) {
            showNotification('Invalid folder name. Use only letters, numbers, spaces, and -_.', 'error');
            return;
        }
        
        const csrfToken = getCSRFToken();
        fetch('/api/folder', {
            method: 'POST',
            headers: { 
                'Content-Type': 'application/json',
                'X-CSRF-Token': csrfToken || ''
            },
            body: JSON.stringify({ path: name })
        })
        .then(response => {
            if (response.ok) {
                showNotification(`Folder "${name}" created successfully!`, 'success');
                fetchCSRFToken();
                setTimeout(() => location.reload(), 500);
            } else {
                showNotification('Failed to create folder.', 'error');
                fetchCSRFToken();
            }
        })
        .catch(err => {
            showNotification('Error creating folder.', 'error');
            fetchCSRFToken();
        });
    }

    function initializeSelection() {
        const fileItems = elements.fileContainer.querySelectorAll('.file-item:not(.file-item-parent)');
        
        fileItems.forEach((item, index) => {
            const checkbox = document.createElement('input');
            checkbox.type = 'checkbox';
            checkbox.className = 'file-checkbox';
            checkbox.style.cssText = `
                position: absolute;
                left: 10px;
                top: 50%;
                transform: translateY(-50%);
                z-index: 2;
                cursor: pointer;
                display: none;
            `;
            
            item.dataset.index = index;
            
            item.style.position = 'relative';
            item.insertBefore(checkbox, item.firstChild);
            
            checkbox.addEventListener('change', (e) => {
                e.stopPropagation();
                handleSelectionChange(item, checkbox.checked, index, e.shiftKey);
            });
            
            item.addEventListener('click', (e) => {
                if (state.isSelectionMode && !e.ctrlKey && !e.metaKey) {
                    e.preventDefault();
                    checkbox.checked = !checkbox.checked;
                    handleSelectionChange(item, checkbox.checked, index, e.shiftKey);
                }
            });
        });
    }
    
    function handleSelectionChange(item, isSelected, index, isShiftKey) {
        const path = item.getAttribute('href') || item.dataset.path;
        
        if (isShiftKey && state.lastSelectedIndex !== -1) {
            const start = Math.min(state.lastSelectedIndex, index);
            const end = Math.max(state.lastSelectedIndex, index);
            const fileItems = elements.fileContainer.querySelectorAll('.file-item:not(.file-item-parent)');
            
            for (let i = start; i <= end; i++) {
                const fileItem = fileItems[i];
                const checkbox = fileItem.querySelector('.file-checkbox');
                const itemPath = fileItem.getAttribute('href') || fileItem.dataset.path;
                
                if (checkbox && !checkbox.checked) {
                    checkbox.checked = true;
                    fileItem.classList.add('selected');
                    state.selectedFiles.add(itemPath);
                }
            }
        } else {
            if (isSelected) {
                item.classList.add('selected');
                state.selectedFiles.add(path);
                state.lastSelectedIndex = index;
            } else {
                item.classList.remove('selected');
                state.selectedFiles.delete(path);
            }
        }
        
        updateSelectionUI();
    }
    
    function toggleSelectionMode() {
        state.isSelectionMode = !state.isSelectionMode;
        const checkboxes = elements.fileContainer.querySelectorAll('.file-checkbox');
        const multiSelectBtn = document.getElementById('multiSelectBtn');
        
        if (state.isSelectionMode) {
            elements.fileContainer.classList.add('selection-mode');
            checkboxes.forEach(cb => cb.style.display = 'block');
            if (multiSelectBtn) multiSelectBtn.classList.add('active');
            showSelectionToolbar();
        } else {
            elements.fileContainer.classList.remove('selection-mode');
            checkboxes.forEach(cb => {
                cb.style.display = 'none';
                cb.checked = false;
            });
            if (multiSelectBtn) multiSelectBtn.classList.remove('active');
            clearSelection();
            hideSelectionToolbar();
        }
    }
    
    function clearSelection() {
        state.selectedFiles.clear();
        state.lastSelectedIndex = -1;
        const selectedItems = elements.fileContainer.querySelectorAll('.file-item.selected');
        selectedItems.forEach(item => {
            item.classList.remove('selected');
            const checkbox = item.querySelector('.file-checkbox');
            if (checkbox) checkbox.checked = false;
        });
        updateSelectionUI();
    }
    
    function selectAll() {
        const fileItems = elements.fileContainer.querySelectorAll('.file-item:not(.file-item-parent)');
        fileItems.forEach(item => {
            const checkbox = item.querySelector('.file-checkbox');
            const path = item.getAttribute('href') || item.dataset.path;
            if (checkbox && !checkbox.checked) {
                checkbox.checked = true;
                item.classList.add('selected');
                state.selectedFiles.add(path);
            }
        });
        updateSelectionUI();
    }
    
    function updateSelectionUI() {
        const toolbar = document.getElementById('selectionToolbar');
        if (toolbar) {
            const count = state.selectedFiles.size;
            const countEl = toolbar.querySelector('.selection-count');
            if (countEl) {
                countEl.textContent = count === 0 ? 'No files selected' : 
                    count === 1 ? '1 file selected' : `${count} files selected`;
            }
            
            const downloadBtn = toolbar.querySelector('.download-selected');
            if (downloadBtn) {
                downloadBtn.disabled = count === 0;
            }
        }
    }
    
    function showSelectionToolbar() {
        let toolbar = document.getElementById('selectionToolbar');
        if (!toolbar) {
            toolbar = document.createElement('div');
            toolbar.id = 'selectionToolbar';
            toolbar.className = 'selection-toolbar';
            toolbar.style.cssText = `
                position: fixed;
                bottom: 20px;
                left: 50%;
                transform: translateX(-50%);
                background: var(--color-primary);
                color: white;
                padding: 12px 24px;
                border-radius: 8px;
                box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
                display: flex;
                align-items: center;
                gap: 16px;
                z-index: 1000;
            `;
            
            toolbar.innerHTML = `
                <span class="selection-count">No files selected</span>
                <button class="btn-small select-all" style="background: rgba(255,255,255,0.2); border: none; color: white; padding: 6px 12px; border-radius: 4px; cursor: pointer;">
                    Select All
                </button>
                <button class="btn-small clear-selection" style="background: rgba(255,255,255,0.2); border: none; color: white; padding: 6px 12px; border-radius: 4px; cursor: pointer;">
                    Clear
                </button>
                <button class="btn-small download-selected" style="background: white; border: none; color: var(--color-primary); padding: 6px 16px; border-radius: 4px; cursor: pointer; font-weight: 600;" disabled>
                    Download as ZIP
                </button>
                <button class="btn-small close-selection" style="background: transparent; border: none; color: white; padding: 6px; cursor: pointer; font-size: 20px;">
                    ×
                </button>
            `;
            
            document.body.appendChild(toolbar);
            
            toolbar.querySelector('.select-all').addEventListener('click', selectAll);
            toolbar.querySelector('.clear-selection').addEventListener('click', clearSelection);
            toolbar.querySelector('.download-selected').addEventListener('click', downloadSelectedAsZip);
            toolbar.querySelector('.close-selection').addEventListener('click', toggleSelectionMode);
        }
        
        toolbar.style.display = 'flex';
    }
    
    function hideSelectionToolbar() {
        const toolbar = document.getElementById('selectionToolbar');
        if (toolbar) {
            toolbar.style.display = 'none';
        }
    }
    
    function downloadSelectedAsZip() {
        if (state.selectedFiles.size === 0) return;
        
        const paths = Array.from(state.selectedFiles);
        
        if (paths.length === 1) {
            const link = document.querySelector(`.file-item[href="${paths[0]}"]`);
            if (link && link.dataset.type !== 'folder') {
                window.location.href = paths[0];
                return;
            }
        }
        
        const csrfToken = getCSRFToken();
        const zipName = paths.length === 1 ? 
            paths[0].split('/').pop() + '.zip' : 
            `download_${Date.now()}.zip`;
        
        showNotification('Preparing ZIP download...', 'info');
        
        fetch('/api/zip', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'X-CSRF-Token': csrfToken || ''
            },
            body: JSON.stringify({
                paths: paths,
                name: zipName
            })
        })
        .then(response => {
            if (!response.ok) {
                throw new Error('Failed to create ZIP');
            }
            return response.blob();
        })
        .then(blob => {
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = zipName;
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
            URL.revokeObjectURL(url);
            
            showNotification('ZIP download started!', 'success');
            fetchCSRFToken();
        })
        .catch(err => {
            showNotification('Failed to create ZIP download', 'error');
            console.error('ZIP download error:', err);
            fetchCSRFToken();
        });
    }
    
    function setupKeyboardShortcuts() {
        document.addEventListener('keydown', (e) => {
            if ((e.ctrlKey || e.metaKey) && e.key === 'f') {
                e.preventDefault();
                elements.searchInput.focus();
            }
            
            if ((e.ctrlKey || e.metaKey) && e.key === 'u') {
                e.preventDefault();
                elements.uploadInput.click();
            }
            
            if ((e.ctrlKey || e.metaKey) && e.key === 'n') {
                e.preventDefault();
                createNewFolder();
            }
            
            if ((e.ctrlKey || e.metaKey) && e.key === 'a' && state.isSelectionMode) {
                e.preventDefault();
                selectAll();
            }
            
            if ((e.ctrlKey || e.metaKey) && e.key === 's') {
                e.preventDefault();
                toggleSelectionMode();
            }
            
            if ((e.ctrlKey || e.metaKey) && e.key === 'd' && state.isSelectionMode) {
                e.preventDefault();
                downloadSelectedAsZip();
            }
            
            if (e.key === 'Escape') {
                if (elements.previewModal.style.display !== 'none') {
                    elements.previewModal.style.display = 'none';
                } else if (state.isSelectionMode) {
                    toggleSelectionMode();
                }
            }
        });
    }

    function setupEventListeners() {
        elements.themeToggle?.addEventListener('click', toggleTheme);
        
        elements.viewToggle?.addEventListener('click', toggleViewMode);
        
        elements.layoutToggle?.addEventListener('click', toggleLayoutMode);
        
        elements.searchInput?.addEventListener('input', debounce(handleSearch, 300));
        
        elements.uploadInput?.addEventListener('change', (e) => {
            handleFileSelect(e.target.files);
        });
        
        elements.uploadCancel?.addEventListener('click', cancelUpload);
        
        elements.fileContainer?.addEventListener('click', (e) => {
            if (e.altKey || e.metaKey) {
                previewFile(e);
            }
        });
        
        elements.previewClose?.addEventListener('click', () => {
            elements.previewModal.style.display = 'none';
        });
        
        elements.newFolderBtn?.addEventListener('click', createNewFolder);
        
        const multiSelectBtn = document.getElementById('multiSelectBtn');
        multiSelectBtn?.addEventListener('click', toggleSelectionMode);
    }

    function debounce(func, wait) {
        let timeout;
        return function executedFunction(...args) {
            const later = () => {
                clearTimeout(timeout);
                func(...args);
            };
            clearTimeout(timeout);
            timeout = setTimeout(later, wait);
        };
    }

    function formatSpeed(bytesPerSecond) {
        if (bytesPerSecond < 1024) {
            return Math.round(bytesPerSecond) + ' B/s';
        } else if (bytesPerSecond < 1024 * 1024) {
            return Math.round(bytesPerSecond / 1024) + ' KB/s';
        } else {
            return Math.round(bytesPerSecond / (1024 * 1024)) + ' MB/s';
        }
    }

    function escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    function showNotification(message, type = 'info') {
        const colors = {
            success: '#059669',
            error: '#dc2626',
            info: '#1d4ed8'
        };
        
        const notification = document.createElement('div');
        notification.style.cssText = `
            position: fixed;
            top: 20px;
            right: 20px;
            padding: 12px 20px;
            background: ${colors[type]};
            color: white;
            border-radius: 8px;
            box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
            z-index: 1001;
            animation: slideIn 0.3s ease;
        `;
        notification.textContent = message;
        document.body.appendChild(notification);
        
        setTimeout(() => {
            notification.style.animation = 'slideOut 0.3s ease';
            setTimeout(() => notification.remove(), 300);
        }, 3000);
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }
})();