/**
 * Silhouette-DB Architecture Animation
 * Creates an animated visualization of the system architecture
 */

class ArchitectureAnimation {
    constructor(canvasId) {
        this.canvas = document.getElementById(canvasId);
        if (!this.canvas) return;
        
        this.ctx = this.canvas.getContext('2d');
        this.particles = [];
        this.connections = [];
        this.animationFrame = null;
        this.time = 0;
        
        // Colors matching CSS theme
        this.colors = {
            primary: '#00f5d4',
            secondary: '#9b5de5',
            tertiary: '#f72585',
            warning: '#ffd60a',
            bg: '#0a0a0f'
        };
        
        // Initialize
        this.init();
        this.setupEventListeners();
        this.animate();
    }
    
    init() {
        this.resize();
        this.createParticles();
        this.createConnections();
    }
    
    resize() {
        const container = this.canvas.parentElement;
        const rect = container.getBoundingClientRect();
        
        // Set canvas size with device pixel ratio for sharp rendering
        const dpr = window.devicePixelRatio || 1;
        this.canvas.width = rect.width * dpr;
        this.canvas.height = rect.height * dpr;
        this.canvas.style.width = `${rect.width}px`;
        this.canvas.style.height = `${rect.height}px`;
        this.ctx.scale(dpr, dpr);
        
        this.width = rect.width;
        this.height = rect.height;
    }
    
    setupEventListeners() {
        window.addEventListener('resize', () => {
            this.resize();
            this.createParticles();
            this.createConnections();
        });
        
        // Interactive node highlighting
        const nodes = document.querySelectorAll('.arch-node, .arch-component');
        nodes.forEach(node => {
            node.addEventListener('mouseenter', () => {
                this.highlightConnections(node.dataset.node || node.dataset.component);
            });
            node.addEventListener('mouseleave', () => {
                this.resetHighlight();
            });
        });
    }
    
    createParticles() {
        this.particles = [];
        
        // Create floating background particles
        for (let i = 0; i < 30; i++) {
            this.particles.push({
                x: Math.random() * this.width,
                y: Math.random() * this.height,
                size: Math.random() * 3 + 1,
                speedX: (Math.random() - 0.5) * 0.5,
                speedY: (Math.random() - 0.5) * 0.5,
                opacity: Math.random() * 0.5 + 0.2,
                color: this.getRandomColor()
            });
        }
    }
    
    createConnections() {
        this.connections = [];
        
        // Define connection paths
        const connectionDefs = [
            // Workers to API layer
            { from: { x: 0.15, y: 0.1 }, to: { x: 0.5, y: 0.25 }, type: 'data' },
            { from: { x: 0.38, y: 0.1 }, to: { x: 0.5, y: 0.25 }, type: 'data' },
            { from: { x: 0.62, y: 0.1 }, to: { x: 0.5, y: 0.25 }, type: 'data' },
            { from: { x: 0.85, y: 0.1 }, to: { x: 0.5, y: 0.25 }, type: 'data' },
            
            // API to Coordination layer
            { from: { x: 0.5, y: 0.28 }, to: { x: 0.35, y: 0.5 }, type: 'pir' },
            { from: { x: 0.5, y: 0.28 }, to: { x: 0.65, y: 0.5 }, type: 'pir' },
            
            // Coordination to Raft
            { from: { x: 0.35, y: 0.55 }, to: { x: 0.5, y: 0.75 }, type: 'data' },
            { from: { x: 0.65, y: 0.55 }, to: { x: 0.5, y: 0.75 }, type: 'data' },
            
            // Raft replication
            { from: { x: 0.35, y: 0.85 }, to: { x: 0.5, y: 0.85 }, type: 'replication' },
            { from: { x: 0.5, y: 0.85 }, to: { x: 0.65, y: 0.85 }, type: 'replication' },
        ];
        
        connectionDefs.forEach(def => {
            this.connections.push({
                from: { x: def.from.x * this.width, y: def.from.y * this.height },
                to: { x: def.to.x * this.width, y: def.to.y * this.height },
                type: def.type,
                particles: [],
                highlighted: false
            });
        });
        
        // Initialize connection particles
        this.connections.forEach(conn => {
            for (let i = 0; i < 3; i++) {
                conn.particles.push({
                    progress: Math.random(),
                    speed: 0.005 + Math.random() * 0.005,
                    size: 3 + Math.random() * 2
                });
            }
        });
    }
    
    getRandomColor() {
        const colors = [this.colors.primary, this.colors.secondary, this.colors.tertiary];
        return colors[Math.floor(Math.random() * colors.length)];
    }
    
    getConnectionColor(type) {
        switch (type) {
            case 'data': return this.colors.primary;
            case 'replication': return this.colors.warning;
            case 'pir': return this.colors.secondary;
            default: return this.colors.primary;
        }
    }
    
    highlightConnections(nodeId) {
        // Highlight connections related to the hovered node
        this.connections.forEach(conn => {
            conn.highlighted = true;
        });
    }
    
    resetHighlight() {
        this.connections.forEach(conn => {
            conn.highlighted = false;
        });
    }
    
    drawBackground() {
        // Gradient background
        const gradient = this.ctx.createRadialGradient(
            this.width / 2, this.height / 2, 0,
            this.width / 2, this.height / 2, this.width / 2
        );
        gradient.addColorStop(0, 'rgba(0, 245, 212, 0.03)');
        gradient.addColorStop(1, 'transparent');
        
        this.ctx.fillStyle = gradient;
        this.ctx.fillRect(0, 0, this.width, this.height);
    }
    
    drawParticles() {
        this.particles.forEach(particle => {
            // Update position
            particle.x += particle.speedX;
            particle.y += particle.speedY;
            
            // Wrap around edges
            if (particle.x < 0) particle.x = this.width;
            if (particle.x > this.width) particle.x = 0;
            if (particle.y < 0) particle.y = this.height;
            if (particle.y > this.height) particle.y = 0;
            
            // Draw particle
            this.ctx.beginPath();
            this.ctx.arc(particle.x, particle.y, particle.size, 0, Math.PI * 2);
            this.ctx.fillStyle = this.hexToRgba(particle.color, particle.opacity);
            this.ctx.fill();
        });
    }
    
    drawConnections() {
        this.connections.forEach(conn => {
            const color = this.getConnectionColor(conn.type);
            const opacity = conn.highlighted ? 0.4 : 0.15;
            
            // Draw connection line
            this.ctx.beginPath();
            this.ctx.moveTo(conn.from.x, conn.from.y);
            
            // Create curved path
            const midX = (conn.from.x + conn.to.x) / 2;
            const midY = (conn.from.y + conn.to.y) / 2;
            const offsetX = (conn.to.y - conn.from.y) * 0.1;
            const offsetY = (conn.from.x - conn.to.x) * 0.1;
            
            this.ctx.quadraticCurveTo(
                midX + offsetX, midY + offsetY,
                conn.to.x, conn.to.y
            );
            
            this.ctx.strokeStyle = this.hexToRgba(color, opacity);
            this.ctx.lineWidth = 1;
            this.ctx.stroke();
            
            // Draw animated particles along the path
            conn.particles.forEach(particle => {
                particle.progress += particle.speed;
                if (particle.progress > 1) particle.progress = 0;
                
                // Calculate position along curve
                const t = particle.progress;
                const x = Math.pow(1 - t, 2) * conn.from.x + 
                         2 * (1 - t) * t * (midX + offsetX) + 
                         Math.pow(t, 2) * conn.to.x;
                const y = Math.pow(1 - t, 2) * conn.from.y + 
                         2 * (1 - t) * t * (midY + offsetY) + 
                         Math.pow(t, 2) * conn.to.y;
                
                // Draw particle with glow
                const particleOpacity = conn.highlighted ? 1 : 0.6;
                
                // Glow effect
                const gradient = this.ctx.createRadialGradient(x, y, 0, x, y, particle.size * 3);
                gradient.addColorStop(0, this.hexToRgba(color, particleOpacity * 0.5));
                gradient.addColorStop(1, 'transparent');
                
                this.ctx.beginPath();
                this.ctx.arc(x, y, particle.size * 3, 0, Math.PI * 2);
                this.ctx.fillStyle = gradient;
                this.ctx.fill();
                
                // Core particle
                this.ctx.beginPath();
                this.ctx.arc(x, y, particle.size, 0, Math.PI * 2);
                this.ctx.fillStyle = this.hexToRgba(color, particleOpacity);
                this.ctx.fill();
            });
        });
    }
    
    drawPulseRings() {
        // Draw pulsing rings at key nodes
        const nodes = [
            { x: this.width * 0.5, y: this.height * 0.5, color: this.colors.secondary },
            { x: this.width * 0.35, y: this.height * 0.85, color: this.colors.warning },
        ];
        
        nodes.forEach(node => {
            const pulse = (Math.sin(this.time * 0.02) + 1) / 2;
            const radius = 20 + pulse * 20;
            
            this.ctx.beginPath();
            this.ctx.arc(node.x, node.y, radius, 0, Math.PI * 2);
            this.ctx.strokeStyle = this.hexToRgba(node.color, 0.1 + pulse * 0.1);
            this.ctx.lineWidth = 2;
            this.ctx.stroke();
        });
    }
    
    hexToRgba(hex, alpha) {
        const r = parseInt(hex.slice(1, 3), 16);
        const g = parseInt(hex.slice(3, 5), 16);
        const b = parseInt(hex.slice(5, 7), 16);
        return `rgba(${r}, ${g}, ${b}, ${alpha})`;
    }
    
    animate() {
        this.time++;
        
        // Clear canvas
        this.ctx.clearRect(0, 0, this.width, this.height);
        
        // Draw layers
        this.drawBackground();
        this.drawParticles();
        this.drawConnections();
        this.drawPulseRings();
        
        // Continue animation
        this.animationFrame = requestAnimationFrame(() => this.animate());
    }
    
    destroy() {
        if (this.animationFrame) {
            cancelAnimationFrame(this.animationFrame);
        }
    }
}

// Data Flow Animation for demonstrating round-based execution
class DataFlowAnimation {
    constructor() {
        this.flowSteps = [
            { label: 'StartRound', description: 'Workers initiate new round' },
            { label: 'PublishValues', description: 'Workers submit key-value pairs' },
            { label: 'Aggregation', description: 'Leader aggregates all pairs' },
            { label: 'OKVS Encode', description: 'Pairs encoded into oblivious structure' },
            { label: 'PIR Setup', description: 'FrodoPIR server created from OKVS' },
            { label: 'GetValue', description: 'Workers query via PIR' },
            { label: 'Response', description: 'Oblivious response returned' }
        ];
        
        this.currentStep = 0;
        this.init();
    }
    
    init() {
        // Create flow indicator if element exists
        const container = document.querySelector('.data-flow-indicator');
        if (!container) return;
        
        this.render(container);
        this.animate();
    }
    
    render(container) {
        const html = this.flowSteps.map((step, index) => `
            <div class="flow-step ${index === 0 ? 'active' : ''}" data-step="${index}">
                <div class="flow-dot"></div>
                <div class="flow-label">${step.label}</div>
            </div>
        `).join('<div class="flow-connector"></div>');
        
        container.innerHTML = html;
    }
    
    animate() {
        setInterval(() => {
            const steps = document.querySelectorAll('.flow-step');
            steps.forEach(step => step.classList.remove('active'));
            
            this.currentStep = (this.currentStep + 1) % this.flowSteps.length;
            steps[this.currentStep]?.classList.add('active');
        }, 2000);
    }
}

// Initialize when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
    // Check if we're on a page with the architecture canvas
    if (document.getElementById('architectureCanvas')) {
        new ArchitectureAnimation('architectureCanvas');
    }
    
    // Initialize data flow animation if present
    new DataFlowAnimation();
});

// Export for potential module usage
if (typeof module !== 'undefined' && module.exports) {
    module.exports = { ArchitectureAnimation, DataFlowAnimation };
}

