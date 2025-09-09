const addWarpMenu = function(){
    const warpScript = document.createElement("script");
    warpScript.type = "text/javascript";
    // Update this version whenever warp menu is updated
    warpScript.src = "/warp/warp.js?nginx={{.Values.controllerManager.manager.image.tag}}";

    const urlScript = document.createElement("script");

    const firstScriptTag = document.getElementsByTagName("script")[0];
    firstScriptTag.parentNode.insertBefore(urlScript, firstScriptTag);
    firstScriptTag.parentNode.insertBefore(warpScript, firstScriptTag);
}

// This variable is used inside the warp menu script
// Update this version whenever warp menu is updated
const cesWarpMenuWarpCssUrl = "/warp/warp.css?nginx={{.Values.controllerManager.manager.image.tag}}";
addWarpMenu();