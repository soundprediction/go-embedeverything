use libc::{c_char, c_float, size_t, c_int};
use std::ffi::CStr;
use embed_anything::embeddings::embed::Embedder;
use embed_anything::reranker::model::Reranker;
use tokio::runtime::Runtime;

#[no_mangle]
pub extern "C" fn init_onnx_runtime(path: *const c_char) -> c_int {
    let p = unsafe { CStr::from_ptr(path).to_str() };
    match p {
        Ok(path_str) => {
             match ort::init_from(path_str).commit() {
                 Ok(_) => 0,
                 Err(e) => {
                     eprintln!("ORT init error: {:?}", e);
                     1
                 }
             }
        },
        Err(_) => 1
    }
}

pub struct EmbedderWrapper {
    pub inner: Embedder,
    pub runtime: Runtime,
}

pub struct RerankerWrapper {
    pub inner: Reranker,
    pub runtime: Runtime,
}

#[repr(C)]
pub struct EmbeddingVector {
    pub data: *mut c_float,
    pub len: size_t,
}

#[repr(C)]
pub struct BatchEmbeddingResult {
    pub vectors: *mut EmbeddingVector,
    pub count: size_t,
}

#[repr(C)]
pub struct RerankResult {
    pub index: size_t,
    pub score: c_float,
    pub text: *mut c_char,
}

#[repr(C)]
pub struct BatchRerankResult {
    pub results: *mut RerankResult,
    pub count: size_t,
}

#[no_mangle]
pub extern "C" fn new_embedder(model_id: *const c_char, architecture: *const c_char) -> *mut EmbedderWrapper {
    let m_id = unsafe { CStr::from_ptr(model_id).to_string_lossy() };
    let arch = unsafe { CStr::from_ptr(architecture).to_string_lossy() };
    
    let runtime = match Runtime::new() {
        Ok(rt) => rt,
        Err(_) => return std::ptr::null_mut(),
    };

    match Embedder::from_pretrained_hf(&arch, &m_id, None, None, None) {
        Ok(e) => Box::into_raw(Box::new(EmbedderWrapper { inner: e, runtime })),
        Err(e) => {
            eprintln!("Error creating embedder: {:?}", e);
            std::ptr::null_mut()
        },
    }
}

#[no_mangle]
pub unsafe extern "C" fn embed_text_batch(wrapper: *mut EmbedderWrapper, texts: *const *const c_char, count: size_t) -> *mut BatchEmbeddingResult {
    if wrapper.is_null() { return std::ptr::null_mut(); }
    let wrapper_ref = &mut *wrapper;
    
    let slice = std::slice::from_raw_parts(texts, count as usize);
    let mut rust_strings = Vec::with_capacity(count as usize);
    for &ptr in slice {
        if ptr.is_null() {
            rust_strings.push("".to_string());
        } else {
            rust_strings.push(CStr::from_ptr(ptr).to_string_lossy().to_string());
        }
    }
    
    let refs: Vec<&str> = rust_strings.iter().map(|s| s.as_str()).collect();

    let res = wrapper_ref.runtime.block_on(async {
        wrapper_ref.inner.embed(&refs, None, None).await
    });

    match res {
        Ok(output) => {
             let mut out_vectors = Vec::with_capacity(output.len());
             
             for item in output {
                 match item {
                     embed_anything::embeddings::embed::EmbeddingResult::DenseVector(v) => {
                         let mut vec_clone = v.clone();
                         let len = vec_clone.len();
                         let ptr = vec_clone.as_mut_ptr();
                         std::mem::forget(vec_clone);
                         
                         out_vectors.push(EmbeddingVector {
                             data: ptr,
                             len: len as size_t,
                         });
                     },
                     _ => {
                         out_vectors.push(EmbeddingVector {
                             data: std::ptr::null_mut(),
                             len: 0,
                         });
                     }
                 }
             }
             
             let count = out_vectors.len();
             let ptr = out_vectors.as_mut_ptr();
             std::mem::forget(out_vectors);
             
             Box::into_raw(Box::new(BatchEmbeddingResult {
                 vectors: ptr,
                 count: count as size_t,
             }))
        },
        Err(e) => {
             eprintln!("Embed batch error: {:?}", e);
             std::ptr::null_mut()
        }
    }
}

#[no_mangle]
pub extern "C" fn new_reranker(model_id: *const c_char) -> *mut RerankerWrapper {
    let m_id = unsafe { CStr::from_ptr(model_id).to_string_lossy() };
    
    let runtime = match Runtime::new() {
        Ok(rt) => rt,
        Err(_) => return std::ptr::null_mut(),
    };

    // Assuming default revision
    // Reranker::new(model_id, revision, dtype, path_in_repo)
    // We default to F32.
    match Reranker::new(&m_id, None, embed_anything::Dtype::F32, None) {
        Ok(r) => Box::into_raw(Box::new(RerankerWrapper { inner: r, runtime })),
        Err(e) => {
            eprintln!("Error creating reranker: {:?}", e);
            std::ptr::null_mut()
        },
    }
}

#[no_mangle]
pub unsafe extern "C" fn rerank_documents(wrapper: *mut RerankerWrapper, query: *const c_char, documents: *const *const c_char, count: size_t) -> *mut BatchRerankResult {
    if wrapper.is_null() { return std::ptr::null_mut(); }
    let wrapper_ref = &mut *wrapper;
    
    let q = CStr::from_ptr(query).to_string_lossy();
    
    let slice = std::slice::from_raw_parts(documents, count as usize);
    let mut docs_owned = Vec::with_capacity(count as usize);
    for &ptr in slice {
        if !ptr.is_null() {
            docs_owned.push(CStr::from_ptr(ptr).to_string_lossy().to_string());
        } else {
            docs_owned.push("".to_string());
        }
    }
    // Create refs must live as long as the call
    let docs_refs: Vec<&str> = docs_owned.iter().map(|s| s.as_str()).collect();

    let res = wrapper_ref.inner.rerank(vec![&q], docs_refs, 32); 
    
    match res {
        Ok(results) => {
             // results is Vec<RerankerResult>.
             if let Some(first_result) = results.first() {
                 let mut out_results = Vec::with_capacity(first_result.documents.len());
                 
                 for (i, item) in first_result.documents.iter().enumerate() {
                     let c_text = std::ffi::CString::new(item.document.clone()).unwrap().into_raw();
                     out_results.push(RerankResult {
                         index: i as size_t,
                         score: item.relevance_score as c_float,
                         text: c_text,
                     });
                 }
                 
                 // Sort by score descending to be useful
                 out_results.sort_by(|a, b| b.score.partial_cmp(&a.score).unwrap_or(std::cmp::Ordering::Equal));
                 
                 let count = out_results.len();
                 let ptr = out_results.as_mut_ptr();
                 std::mem::forget(out_results);
                 
                 Box::into_raw(Box::new(BatchRerankResult {
                     results: ptr,
                     count: count as size_t,
                 }))
             } else {
                 Box::into_raw(Box::new(BatchRerankResult {
                     results: std::ptr::null_mut(),
                     count: 0,
                 }))
             }
        },
        Err(e) => {
             eprintln!("Rerank error: {:?}", e);
             std::ptr::null_mut()
        }
    }
}


#[no_mangle]
pub extern "C" fn free_embedder(wrapper: *mut EmbedderWrapper) {
    if !wrapper.is_null() {
        unsafe { let _ = Box::from_raw(wrapper); }
    }
}

#[no_mangle]
pub unsafe extern "C" fn free_batch_result(result: *mut BatchEmbeddingResult) {
    if !result.is_null() {
        let batch = Box::from_raw(result);
        let vectors = Vec::from_raw_parts(batch.vectors, batch.count as usize, batch.count as usize);
        for v in vectors {
            if !v.data.is_null() {
                let _ = Vec::from_raw_parts(v.data, v.len as usize, v.len as usize);
            }
        }
    }
}

#[no_mangle]
pub extern "C" fn free_reranker(wrapper: *mut RerankerWrapper) {
    if !wrapper.is_null() {
        unsafe { let _ = Box::from_raw(wrapper); }
    }
}

#[no_mangle]
pub unsafe extern "C" fn free_rerank_result(result: *mut BatchRerankResult) {
     if !result.is_null() {
         let batch = Box::from_raw(result);
         let vectors = Vec::from_raw_parts(batch.results, batch.count as usize, batch.count as usize);
         for v in vectors {
             if !v.text.is_null() {
                 let _ = std::ffi::CString::from_raw(v.text);
             }
         }
     }
}
