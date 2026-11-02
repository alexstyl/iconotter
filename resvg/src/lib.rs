//! WebAssembly entry points that Icon Otter calls through wazero.

use resvg::{tiny_skia, usvg};

/// Reserves `len` bytes that the host fills with SVG data.
#[no_mangle]
pub extern "C" fn alloc(len: usize) -> *mut u8 {
    let mut buffer = Vec::<u8>::with_capacity(len);
    let ptr = buffer.as_mut_ptr();
    std::mem::forget(buffer);
    ptr
}

/// Releases a buffer returned by `alloc` or `render`.
#[no_mangle]
pub unsafe extern "C" fn dealloc(ptr: *mut u8, len: usize) {
    drop(Vec::from_raw_parts(ptr, 0, len));
}

/// Renders the SVG stretched to `width`×`height` and returns premultiplied RGBA
/// pixels (`width * height * 4` bytes), or null when the SVG cannot be rendered.
#[no_mangle]
pub unsafe extern "C" fn render(svg_ptr: *const u8, svg_len: usize, width: u32, height: u32) -> *mut u8 {
    let svg = std::slice::from_raw_parts(svg_ptr, svg_len);
    let Ok(tree) = usvg::Tree::from_data(svg, &usvg::Options::default()) else {
        return std::ptr::null_mut();
    };
    let Some(mut pixmap) = tiny_skia::Pixmap::new(width, height) else {
        return std::ptr::null_mut();
    };
    let size = tree.size();
    let transform = tiny_skia::Transform::from_scale(
        width as f32 / size.width(),
        height as f32 / size.height(),
    );
    resvg::render(&tree, transform, &mut pixmap.as_mut());
    let mut pixels = pixmap.take();
    pixels.shrink_to_fit();
    let ptr = pixels.as_mut_ptr();
    std::mem::forget(pixels);
    ptr
}
