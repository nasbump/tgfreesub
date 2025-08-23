class MessageLoader {
    constructor() {
        this.offset = 0;
        this.isLoading = false;
        this.hasMore = true;
        this.observer = null;
        this.init();
    }

    init() {
        this.setupIntersectionObserver();
        this.loadMessages();
    }

    setupIntersectionObserver() {
        const options = {
            root: null,
            rootMargin: '100px',
            threshold: 0.1
        };

        this.observer = new IntersectionObserver((entries) => {
            entries.forEach(entry => {
                // console.log('Intersection observed:', entry.isIntersecting, 'hasMore:', this.hasMore, 'isLoading:', this.isLoading);
                if (entry.isIntersecting && this.hasMore && !this.isLoading) {
                    // console.log('Loading more messages...');
                    this.loadMessages();
                }
            });
        }, options);

        const endMarker = document.getElementById('end-marker');
        if (endMarker) {
            // 确保end-marker一开始就可见（即使很小）
            endMarker.style.minHeight = '1px';
            this.observer.observe(endMarker);
            console.log('Intersection Observer set up on end-marker');
        } else {
            console.error('end-marker element not found');
        }
    }

    async loadMessages() {
        if (this.isLoading || !this.hasMore) return;

        this.isLoading = true;
        this.showLoading(true);

        try {
            const pageSize = this.getPageSize();
            // 第一次加载时offset为0，之后使用API返回的offset
            const currentOffset = this.offset === 0 ? 0 : this.offset;
            const url = `/subs/list?offset=${currentOffset}&number=${pageSize}`;
            
            const response = await fetch(url);
            const data = await response.json();

            if (data.rtn === 0 && data.items) {
                this.renderMessages(data.items);
                this.offset = data.offset;
                
                if (data.offset < 0) {
                    this.hasMore = false;
                    this.showEndMarker();
                }
            // } else {
                // console.error('API返回错误:', data.msg);
            }
        } catch (error) {
            console.error('加载消息失败:', error);
        } finally {
            this.isLoading = false;
            this.showLoading(false);
        }
    }

    getPageSize() {
        return window.innerWidth >= 768 ? 20 : 10;
    }

    renderMessages(items) {
        const messageList = document.getElementById('message-list');
        
        items.forEach(item => {
            const messageCard = this.createMessageCard(item);
            messageList.appendChild(messageCard);
        });
    }

    createMessageCard(item) {
        const card = document.createElement('div');
        card.className = 'message-card';
        
        // 1. 直接展示 content 字段内容
        const messageContent = document.createElement('div');
        messageContent.className = 'message-content';
        
        // 处理消息内容，将HTML实体转换回正常文本
        let content = item.content || '无内容';
        content = content.replace(/<\/ p>/g, '</p>');
        content = content.replace(/</g, '<').replace(/>/g, '>');
        content = content.replace(/&/g, '&');
        content = content.replace(/"/g, '"');
        content = content.replace(/&#39;/g, "'");
        
        messageContent.innerHTML = content;
        
        // 2. 在下方展示抓取时间
        const messageDate = document.createElement('div');
        messageDate.className = 'message-date';
        messageDate.textContent = `抓取时间：${item.date || '未知时间'}`;
        
        // 3. 展示 name 字段作为超链接指向 url
        const channelInfo = document.createElement('div');
        channelInfo.className = 'channel-info';
        
        const channelLink = document.createElement('a');
        channelLink.className = 'channel-url';
        // 确保URL有https://前缀
        let channelUrl = item.url || '#';
        if (channelUrl !== '#' && !channelUrl.startsWith('https://')) {
            channelUrl = 'https://' + channelUrl;
        }
        channelLink.href = channelUrl;
        channelLink.target = '_blank';
        channelLink.rel = 'noopener noreferrer';
        channelLink.textContent = item.name || '未知频道';
        
        channelInfo.appendChild(channelLink);
        
        card.appendChild(messageContent);
        card.appendChild(messageDate);
        card.appendChild(channelInfo);
        
        return card;
    }

    showLoading(show) {
        const loading = document.getElementById('loading');
        if (loading) {
            loading.style.display = show ? 'block' : 'none';
        }
    }

    showEndMarker() {
        const endMarker = document.getElementById('end-marker');
        if (endMarker) {
            endMarker.classList.add('visible');
        }
    }
}

// 页面加载完成后初始化
document.addEventListener('DOMContentLoaded', () => {
    new MessageLoader();
});

// 窗口大小变化时重新计算页面大小
window.addEventListener('resize', () => {
    // 可以在这里添加响应式处理的逻辑
});
