scionApp
    .factory('userService', ["$http", "$q", function ($http, $q) {
    return {
        // Load the user's data
        userPageData: function () {
            return $http.get('/api/userPageData').then(function (response) {
                console.log(response);
                return response.data;
            });
        },
        // Create SCIONLab VM
        generateSCIONLabVM: function (user) {
            // TODO(ercanucan): compose the URL in a cleaner fashion
            let url = '/api/as/generateVM?isVPN=' + (!user.isNotVPN) + '&scionLabVMIP='
                + user.scionLabVMIP;
            return $http.post(url).then(function (response) {
                console.log(response);
                return response.data;
            });
        },
        // Remove SCIONLab VM
        removeSCIONLabVM: function (user) {
            console.log("Inside remove VM");
            return $http.post('/api/as/removeVM').then(function (response) {
                // The then function here is an opportunity to modify the response
                console.log(response);
                return response.data;
            });
        },

        getUserBuildImages: function() {
            console.log("Get user images");
            return $http.get('/api/imgbuild/user-images').then(function (response) {
                console.log(response);
                return response.data;
            });
        },

        getAvailableImages: function() {
            console.log("Get available images");
            return $http.get('/api/imgbuild/images').then(function (response) {
                console.log(response);
                return response.data;
            });
        },

        startBuildJob: function(imageName) {
            console.log("Start build job");
            return $http.post('/api/imgbuild/create', {"image_name":imageName}).then(function (response) {
                console.log(response);
                return response.data;
            });
        }
    };
}]);
