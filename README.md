# Virus Scanner API

🦠🛡️ High-performance virus scanning service powered by ClamAV and Go FastHTTP.

## 🚀 Live API

**Production URL:** https://virus-scanner.fly.dev/

## 📡 API Endpoints

### POST /scan
Upload and scan files for viruses.

**URL:** `https://virus-scanner.fly.dev/scan`  
**Method:** `POST`  
**Content-Type:** `multipart/form-data`  
**Max File Size:** 100MB  

#### Request Parameters
- `file` (required): The file to scan (form-data)

#### Response Format
```json
{
  "filename": "document.pdf",
  "status": "clean|infected", 
  "threat": "threat_name_if_infected",
  "scan_time": "2025-12-01T06:35:42Z",
  "engine": "ClamAV-Optimized"
}
```

### GET /
Web UI for manual file uploads and testing.

## 🔧 Axios Usage Examples

### Basic File Upload
```javascript
import axios from 'axios';

const scanFile = async (file) => {
  const formData = new FormData();
  formData.append('file', file);
  
  try {
    const response = await axios.post('https://virus-scanner.fly.dev/scan', formData, {
      headers: {
        'Content-Type': 'multipart/form-data'
      }
    });
    
    return response.data;
  } catch (error) {
    console.error('Scan failed:', error.response?.data || error.message);
    throw error;
  }
};
```

### File Upload with Progress Tracking
```javascript
const scanFileWithProgress = async (file, onProgress) => {
  const formData = new FormData();
  formData.append('file', file);
  
  try {
    const response = await axios.post('https://virus-scanner.fly.dev/scan', formData, {
      headers: {
        'Content-Type': 'multipart/form-data'
      },
      onUploadProgress: (progressEvent) => {
        const progress = Math.round((progressEvent.loaded * 100) / progressEvent.total);
        onProgress(progress);
      }
    });
    
    return response.data;
  } catch (error) {
    console.error('Scan failed:', error.response?.data || error.message);
    throw error;
  }
};

// Usage
const file = document.getElementById('fileInput').files[0];
scanFileWithProgress(file, (progress) => {
  console.log(`Upload progress: ${progress}%`);
}).then(result => {
  console.log('Scan result:', result);
});
```

### React Hook Example
```javascript
import { useState } from 'react';
import axios from 'axios';

const useVirusScanner = () => {
  const [scanning, setScanning] = useState(false);
  const [progress, setProgress] = useState(0);
  
  const scanFile = async (file) => {
    setScanning(true);
    setProgress(0);
    
    const formData = new FormData();
    formData.append('file', file);
    
    try {
      const response = await axios.post('https://virus-scanner.fly.dev/scan', formData, {
        headers: {
          'Content-Type': 'multipart/form-data'
        },
        onUploadProgress: (progressEvent) => {
          const progress = Math.round((progressEvent.loaded * 100) / progressEvent.total);
          setProgress(progress);
        }
      });
      
      return response.data;
    } catch (error) {
      throw new Error(error.response?.data || 'Scan failed');
    } finally {
      setScanning(false);
      setProgress(0);
    }
  };
  
  return { scanFile, scanning, progress };
};
```

### Node.js Server Example
```javascript
const axios = require('axios');
const FormData = require('form-data');
const fs = require('fs');

const scanFile = async (filePath) => {
  const form = new FormData();
  form.append('file', fs.createReadStream(filePath));
  
  try {
    const response = await axios.post('https://virus-scanner.fly.dev/scan', form, {
      headers: {
        ...form.getHeaders()
      }
    });
    
    return response.data;
  } catch (error) {
    console.error('Scan failed:', error.response?.data || error.message);
    throw error;
  }
};

// Usage
scanFile('./document.pdf').then(result => {
  console.log('Scan result:', result);
});
```

### Error Handling
```javascript
const scanWithErrorHandling = async (file) => {
  try {
    const result = await scanFile(file);
    
    if (result.status === 'infected') {
      console.warn(`⚠️ Virus detected: ${result.threat}`);
      return { safe: false, threat: result.threat };
    } else {
      console.log('✅ File is clean');
      return { safe: true, threat: null };
    }
  } catch (error) {
    if (error.response?.status === 413) {
      throw new Error('File too large (max 100MB)');
    } else if (error.response?.status === 400) {
      throw new Error('Invalid file format');
    } else {
      throw new Error('Scan service unavailable');
    }
  }
};
```

## 🌐 CORS Support

The API supports CORS for the following domains:
- `https://ecoop-suite.netlify.app`
- `https://ecoop-suite.com`
- `https://www.ecoop-suite.com`
- Development environments (`localhost:3000`, `localhost:3001`, etc.)
- Fly.io deployment domains

## 📝 Response Examples

### Clean File
```json
{
  "filename": "document.pdf",
  "status": "clean",
  "threat": "",
  "scan_time": "2025-12-01T06:35:42Z",
  "engine": "ClamAV-Optimized"
}
```

### Infected File
```json
{
  "filename": "malware.exe",
  "status": "infected", 
  "threat": "Win.Trojan.Generic-1234",
  "scan_time": "2025-12-01T06:35:42Z",
  "engine": "ClamAV-Optimized"
}
```

### Error Response
```json
{
  "error": "File too large",
  "message": "Maximum file size is 100MB"
}
```

## ⚡ Performance Notes

- **Max File Size:** 100MB
- **Concurrent Scans:** Optimized for memory efficiency
- **Response Time:** Typically 2-10 seconds depending on file size
- **High Availability:** Multiple instances with load balancing
- **Memory Optimized:** 2GB RAM per instance

## 🔒 Security Features

- ✅ Real-time virus scanning with ClamAV
- ✅ No file storage (streaming processing)
- ✅ Memory-optimized processing
- ✅ CORS protection
- ✅ Request size limits
- ✅ Production-ready deployment

## 📚 Integration Examples

### Vue.js Component
```vue
<template>
  <div>
    <input type="file" @change="handleFileSelect" ref="fileInput" />
    <button @click="scanFile" :disabled="scanning">
      {{ scanning ? 'Scanning...' : 'Scan File' }}
    </button>
    <div v-if="progress > 0">Progress: {{ progress }}%</div>
    <div v-if="result">
      Status: {{ result.status }}
      <span v-if="result.threat">({{ result.threat }})</span>
    </div>
  </div>
</template>

<script>
import axios from 'axios';

export default {
  data() {
    return {
      selectedFile: null,
      scanning: false,
      progress: 0,
      result: null
    };
  },
  methods: {
    handleFileSelect(event) {
      this.selectedFile = event.target.files[0];
    },
    async scanFile() {
      if (!this.selectedFile) return;
      
      this.scanning = true;
      this.progress = 0;
      this.result = null;
      
      const formData = new FormData();
      formData.append('file', this.selectedFile);
      
      try {
        const response = await axios.post('https://virus-scanner.fly.dev/scan', formData, {
          headers: { 'Content-Type': 'multipart/form-data' },
          onUploadProgress: (e) => {
            this.progress = Math.round((e.loaded * 100) / e.total);
          }
        });
        
        this.result = response.data;
      } catch (error) {
        console.error('Scan failed:', error);
      } finally {
        this.scanning = false;
      }
    }
  }
};
</script>
```

## 🏗️ Architecture

- **Backend:** Go 1.24 + FastHTTP
- **Antivirus:** ClamAV with latest virus definitions  
- **Deployment:** Fly.io with auto-scaling
- **Memory:** 2GB RAM per instance
- **Storage:** Stateless (no file persistence)

---

Built with ❤️ for secure file processing. Deploy your own instance or use the hosted version at https://virus-scanner.fly.dev/