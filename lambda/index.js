var child_process = require('child_process');
var http = require('https');
var fs = require('fs');
var url = require("url");
var path = require("path");

var agent_url = "https://cdn.cloudprowess.com/dv/agent";
var event_file = '/tmp/event.json';
var context_file = '/tmp/context.json';

// download a binary file from a URL and execute it
// TODO: only download the file if we don't have it yet with the same version
var download_and_run = function(my_url, event, context) {

  console.log("Downloading/running agent...");
  // prepare the binary file
  var parsed_url = url.parse(my_url);
  var binary = "/tmp/"+path.basename(parsed_url.pathname);

  try {
    // Check if file exists
    stats = fs.lstatSync(binary);

    if (stats.isFile()) {
      console.log('Binary file '+ binary + ' already exists, running it...');
      run(binary, event, context);

      // delete binary if older than a week
      var now = Date.now();
      var timeDiff = Math.abs(now - stats.mtime.getTime());
      var diffDays = Math.ceil(timeDiff / (1000 * 3600 * 24));
      if (diffDays > 7){
        fs.unlinkSync(binary);
      }
    }
  }
  catch (e) {
    console.log('file is missing, downloading it first');

    var file = fs.createWriteStream(binary);
    // download the binary into a new file
    var request = http.get(my_url, function(response) {
      response.pipe(file);

      // when finished downloading
      file.on('finish', function() {
        file.close(function() {
          // make it executable
          fs.chmodSync(binary, 0755);

          //execute it
          console.log("Running " + binary);
          run(binary, event, context);
        });
      // basic error handling
      }).on('error', function(err) {
        // Delete the file async.
        fs.unlink(dest);
      });
    });
  }
};

// run the file we just downloaded
var run = function(binary_file, event, context){
  console.log("Running " + binary_file);

  // prepare input data based on the SNS event
  fs.writeFileSync(event_file, JSON.stringify(event));
  fs.writeFileSync(context_file, JSON.stringify(context));

  // run the process
  var proc = child_process.spawn(binary_file, ['--event_file', event_file, '--context_file', context_file], { stdio: 'inherit' });

  // handle exit codes
  proc.on('close', function(code){
    if(code !== 0) {
      return context.done(new Error("Process exited with non-zero status code"));
    } else {
      console.log("Process " + binary_file + " exited with zero status code");
      return context.done(null);
    }
  });
};

// main function
exports.handler = function(event, context) {
  download_and_run(agent_url, event, context); // download and run agent after the download is completed
};
