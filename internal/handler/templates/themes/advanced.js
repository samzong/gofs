// GoFS Advanced Theme - Interactive JavaScript
(function() {
    'use strict';

    // State Management
    const state = {
        viewMode: localStorage.getItem('viewMode') || 'grid',
        theme: localStorage.getItem('theme') || 'light',
        layoutMode: localStorage.getItem('layoutMode') || 'centered',
        sortBy: 'name',
        sortOrder: 'asc',
        searchQuery: '',
        uploadProgress: 0,
        uploadXHR: null
    };

    // DOM Elements
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

    // Initialize
    function init() {
        applyTheme(state.theme);
        applyViewMode(state.viewMode);
        applyLayoutMode(state.layoutMode);
        setupEventListeners();
        setupDragAndDrop();
        setupKeyboardShortcuts();
    }

    // Theme Management
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

    // View Mode Management
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

    // Layout Management
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

    // Search Functionality
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
        
        // Update file count
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

    // Upload Functionality
    function handleFileSelect(files) {
        if (files.length === 0) return;
        
        // MVP: Single file upload
        const file = files[0];
        uploadFile(file);
    }

    function uploadFile(file) {
        // Validation
        const maxSize = 100 * 1024 * 1024; // 100MB
        if (file.size > maxSize) {
            showNotification('File too large. Maximum size is 100MB.', 'error');
            return;
        }

        // Show progress UI
        elements.uploadProgress.style.display = 'block';
        elements.uploadFilename.textContent = file.name;
        elements.progressFill.style.width = '0%';
        elements.uploadPercent.textContent = '0%';

        // Create FormData
        const formData = new FormData();
        formData.append('file', file);

        // Create XHR for progress tracking
        const xhr = new XMLHttpRequest();
        state.uploadXHR = xhr;

        // Track upload progress
        const startTime = Date.now();
        let lastLoaded = 0;
        
        xhr.upload.addEventListener('progress', (e) => {
            if (e.lengthComputable) {
                const percentComplete = Math.round((e.loaded / e.total) * 100);
                state.uploadProgress = percentComplete;
                
                // Update UI
                elements.progressFill.style.width = percentComplete + '%';
                elements.uploadPercent.textContent = percentComplete + '%';
                
                // Calculate speed
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

        // Send request
        xhr.open('POST', '/api/upload');
        xhr.send(formData);
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

    // Drag and Drop
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

    // File Preview
    function previewFile(e) {
        const link = e.target.closest('.file-item');
        if (!link || link.classList.contains('file-item-parent')) return;
        
        const isFolder = link.dataset.type === 'folder';
        if (isFolder) return;
        
        const filename = link.dataset.name;
        const ext = filename.split('.').pop().toLowerCase();
        
        // Check if preview is supported
        const imageExts = ['jpg', 'jpeg', 'png', 'gif', 'webp', 'svg'];
        const textExts = ['txt', 'md', 'json', 'js', 'css', 'html', 'xml', 'yml', 'yaml'];
        
        if (!imageExts.includes(ext) && !textExts.includes(ext)) {
            return; // Let default action handle it
        }
        
        e.preventDefault();
        
        // Show modal
        elements.previewModal.style.display = 'flex';
        elements.previewTitle.textContent = filename;
        elements.previewBody.innerHTML = 'Loading...';
        
        const url = link.href;
        
        if (imageExts.includes(ext)) {
            elements.previewBody.innerHTML = `<img src="${url}" style="max-width: 100%; height: auto;">`;
        } else {
            // Fetch text content
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

    // New Folder
    function createNewFolder() {
        const name = prompt('Enter folder name:');
        if (!name) return;
        
        // Validate folder name
        if (!/^[a-zA-Z0-9_\-. ]+$/.test(name)) {
            showNotification('Invalid folder name. Use only letters, numbers, spaces, and -_.', 'error');
            return;
        }
        
        fetch('/api/folder', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path: name })
        })
        .then(response => {
            if (response.ok) {
                showNotification(`Folder "${name}" created successfully!`, 'success');
                setTimeout(() => location.reload(), 500);
            } else {
                showNotification('Failed to create folder.', 'error');
            }
        })
        .catch(err => {
            showNotification('Error creating folder.', 'error');
        });
    }

    // Keyboard Shortcuts
    function setupKeyboardShortcuts() {
        document.addEventListener('keydown', (e) => {
            // Ctrl/Cmd + F: Focus search
            if ((e.ctrlKey || e.metaKey) && e.key === 'f') {
                e.preventDefault();
                elements.searchInput.focus();
            }
            
            // Ctrl/Cmd + U: Upload
            if ((e.ctrlKey || e.metaKey) && e.key === 'u') {
                e.preventDefault();
                elements.uploadInput.click();
            }
            
            // Ctrl/Cmd + N: New folder
            if ((e.ctrlKey || e.metaKey) && e.key === 'n') {
                e.preventDefault();
                createNewFolder();
            }
            
            // Escape: Close modals
            if (e.key === 'Escape') {
                if (elements.previewModal.style.display !== 'none') {
                    elements.previewModal.style.display = 'none';
                }
            }
        });
    }

    // Event Listeners
    function setupEventListeners() {
        // Theme toggle
        elements.themeToggle?.addEventListener('click', toggleTheme);
        
        // View mode toggle
        elements.viewToggle?.addEventListener('click', toggleViewMode);
        
        // Layout toggle
        elements.layoutToggle?.addEventListener('click', toggleLayoutMode);
        
        // Search
        elements.searchInput?.addEventListener('input', debounce(handleSearch, 300));
        
        // Upload
        elements.uploadInput?.addEventListener('change', (e) => {
            handleFileSelect(e.target.files);
        });
        
        elements.uploadCancel?.addEventListener('click', cancelUpload);
        
        // Preview
        elements.fileContainer?.addEventListener('click', (e) => {
            if (e.altKey || e.metaKey) {
                previewFile(e);
            }
        });
        
        elements.previewClose?.addEventListener('click', () => {
            elements.previewModal.style.display = 'none';
        });
        
        // New folder
        elements.newFolderBtn?.addEventListener('click', createNewFolder);
    }

    // Utility Functions
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
        // Simple notification (can be enhanced with a proper notification system)
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

    // Initialize on DOM ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }
})();