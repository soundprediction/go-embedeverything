use libc::{c_char, c_float, size_t};
use std::ffi::CStr;
use embed_anything::embeddings::embed::Embedder;
use tokio::runtime::Runtime;

pub struct EmbedderWrapper {
    pub inner: Embedder,
    pub runtime: Runtime,
}

#[repr(C)]
pub struct EmbeddingVector {
    pub data: *mut c_float,
    pub len: size_t,
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
pub extern "C" fn embed_text(wrapper: *mut EmbedderWrapper, text: *const c_char) -> *mut EmbeddingVector {
    if wrapper.is_null() { return std::ptr::null_mut(); }
    let wrapper_ref = unsafe { &mut *wrapper };
    let txt = unsafe { CStr::from_ptr(text).to_string_lossy() };

    let texts = vec![txt.to_string()];
    let texts_refs: Vec<&str> = texts.iter().map(|s| s.as_str()).collect();

    let res = wrapper_ref.runtime.block_on(async {
        wrapper_ref.inner.embed(&texts_refs, None, None).await
    });

    match res {
        Ok(output) => {
             if let Some(first) = output.first() {
                 match first {
                     embed_anything::embeddings::embed::EmbeddingResult::DenseVector(v) => {
                         let mut vec = v.clone(); 
                         let len = vec.len();
                         let ptr = vec.as_mut_ptr();
                         std::mem::forget(vec);
                         
                         Box::into_raw(Box::new(EmbeddingVector {
                             data: ptr,
                             len: len as size_t,
                         }))
                     },
                     _ => {
                         eprintln!("Unsupported embedding type (expected DenseVector)");
                         std::ptr::null_mut()
                     }
                 }
             } else {
                 std::ptr::null_mut()
             }
        },
        Err(e) => {
             eprintln!("Embed error: {:?}", e);
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
pub extern "C" fn free_embedding_vector(vec: *mut EmbeddingVector) {
    if !vec.is_null() {
        unsafe {
            let v = Box::from_raw(vec);
            let _ = Vec::from_raw_parts(v.data, v.len as usize, v.len as usize);
        }
    }
}
