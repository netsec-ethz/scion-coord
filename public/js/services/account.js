scionApp
    .factory('accountService', ["$http", function ($http) {
    return {
        changePassword: function (pwForm) {
            return $http.post('/api/changePassword', pwForm).then(function (response) {
                console.log(response);
                return response.data;
            });
        }
    };
}]);
